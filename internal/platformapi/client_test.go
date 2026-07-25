package platformapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRegistryAndCredentialRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "access" || password != "secret" {
			t.Fatalf("unexpected basic auth: %q %q %v", user, password, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2-beta/registries":
			_, _ = w.Write([]byte(`{"data":[{"id":"1r1","serverAddress":"registry.example.test"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2-beta/registryCredentials":
			if got := r.URL.Query().Get("registryId"); got != "1r1" {
				t.Fatalf("registryId = %q", got)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"1rc1","registryId":"1r1","links":{"self":"http://untrusted.invalid/credential"}}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2-beta/registries":
			var registry Registry
			if err := json.NewDecoder(r.Body).Decode(&registry); err != nil {
				t.Fatal(err)
			}
			if registry.ServerAddress != "new.example.test" {
				t.Fatalf("unexpected registry payload: %#v", registry)
			}
			_, _ = w.Write([]byte(`{"id":"1r2","serverAddress":"new.example.test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2-beta/registryCredentials":
			var credential RegistryCredential
			if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
				t.Fatal(err)
			}
			if credential.RegistryID != "1r2" || credential.PublicValue != "AWS" {
				t.Fatalf("unexpected credential payload: %#v", credential)
			}
			_, _ = w.Write([]byte(`{"id":"1rc2","registryId":"1r2"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v2-beta/registryCredentials/1rc1":
			var credential RegistryCredential
			if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
				t.Fatal(err)
			}
			if credential.PublicValue != "AWS" || credential.SecretValue != "secret-value" {
				t.Fatalf("unexpected credential payload: %#v", credential)
			}
			_, _ = w.Write([]byte(`{"id":"1rc1","registryId":"1r1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v1", "access", "secret")
	if err != nil {
		t.Fatal(err)
	}
	registries, err := client.Registries.List(&ListOptions{})
	if err != nil || len(registries.Data) != 1 {
		t.Fatalf("registries = %#v, err = %v", registries, err)
	}
	credentials, err := client.RegistryCredentials.List(&ListOptions{Filters: map[string]interface{}{"registryId": "1r1"}})
	if err != nil || len(credentials.Data) != 1 {
		t.Fatalf("credentials = %#v, err = %v", credentials, err)
	}
	_, err = client.RegistryCredentials.Update(&credentials.Data[0], &RegistryCredential{PublicValue: "AWS", SecretValue: "secret-value"})
	if err != nil {
		t.Fatal(err)
	}
	createdRegistry, err := client.Registries.Create(&Registry{ServerAddress: "new.example.test"})
	if err != nil || createdRegistry.ID != "1r2" {
		t.Fatalf("created registry = %#v, err = %v", createdRegistry, err)
	}
	createdCredential, err := client.RegistryCredentials.Create(&RegistryCredential{RegistryID: createdRegistry.ID, PublicValue: "AWS", SecretValue: "secret-value"})
	if err != nil || createdCredential.ID != "1rc2" {
		t.Fatalf("created credential = %#v, err = %v", createdCredential, err)
	}
}

func TestClientRejectsRelativeURL(t *testing.T) {
	if _, err := NewClient("/v1", "access", "secret"); err == nil {
		t.Fatal("expected a relative URL error")
	}
}

func TestClientRequiresScopedCredentials(t *testing.T) {
	for _, test := range []struct {
		access string
		secret string
	}{
		{access: "", secret: "secret"},
		{access: "access", secret: ""},
	} {
		if _, err := NewClient("https://platform.example.test/v1", test.access, test.secret); err == nil {
			t.Fatalf("expected empty scoped credential to be rejected: %#v", test)
		}
	}
}

func TestClientRejectsURLCredentialsAndQuery(t *testing.T) {
	for _, rawURL := range []string{
		"https://user:password@platform.example.test/v1",
		"https://platform.example.test/v1?token=secret",
		"https://platform.example.test/v1#fragment",
	} {
		if _, err := NewClient(rawURL, "access", "secret"); err == nil {
			t.Fatalf("expected URL to be rejected: %s", rawURL)
		}
	}
}

func TestClientDoesNotForwardScopedCredentialsAcrossRedirects(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Store(true)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	client, err := NewClient(redirector.URL+"/v1", "access", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Registries.List(&ListOptions{}); err == nil {
		t.Fatal("expected redirect response to be rejected")
	}
	if redirected.Load() {
		t.Fatal("scoped request followed a redirect")
	}
}
