package main

// Modified by PastureStack contributors for independent maintenance and rebranding.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PastureStack/ecr-credential-sync/internal/platformapi"
	log "github.com/sirupsen/logrus"
)

type synchronizationFixture struct {
	mu                sync.Mutex
	tokenPassword     string
	registryCreated   int
	credentialCreated int
	credentialUpdated int
	storedPassword    string
}

func (fixture *synchronizationFixture) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(request.Header.Get("X-Amz-Target"), "GetAuthorizationToken") {
			if request.Header.Get("Authorization") == "" {
				t.Error("AWS request was not signed")
			}
			fixture.mu.Lock()
			password := fixture.tokenPassword
			fixture.mu.Unlock()
			token := base64.StdEncoding.EncodeToString([]byte("AWS:" + password))
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"authorizationData": []map[string]interface{}{
					{
						"authorizationToken": token,
						"proxyEndpoint":      "https://123456789012.dkr.ecr.us-east-1.amazonaws.com",
					},
				},
			})
			return
		}

		user, password, ok := request.BasicAuth()
		if !ok || user != "platform-access" || password != "platform-secret" {
			t.Errorf("platform request did not use the scoped Basic Auth credentials")
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v2-beta/registries":
			if fixture.registryCreated == 0 {
				_, _ = w.Write([]byte(`{"data":[]}`))
			} else {
				_, _ = w.Write([]byte(`{"data":[{"id":"1r1","serverAddress":"123456789012.dkr.ecr.us-east-1.amazonaws.com"}]}`))
			}
		case request.Method == http.MethodPost && request.URL.Path == "/v2-beta/registries":
			fixture.registryCreated++
			_, _ = w.Write([]byte(`{"id":"1r1","serverAddress":"123456789012.dkr.ecr.us-east-1.amazonaws.com"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/v2-beta/registryCredentials":
			if request.URL.Query().Get("registryId") != "1r1" {
				t.Errorf("unexpected registryId filter: %s", request.URL.RawQuery)
			}
			if fixture.credentialCreated == 0 {
				_, _ = w.Write([]byte(`{"data":[]}`))
			} else {
				_, _ = w.Write([]byte(`{"data":[{"id":"1rc1","registryId":"1r1"}]}`))
			}
		case request.Method == http.MethodPost && request.URL.Path == "/v2-beta/registryCredentials":
			var credential platformapi.RegistryCredential
			if err := json.NewDecoder(request.Body).Decode(&credential); err != nil {
				t.Error(err)
				http.Error(w, `{}`, http.StatusBadRequest)
				return
			}
			fixture.credentialCreated++
			fixture.storedPassword = credential.SecretValue
			_, _ = w.Write([]byte(`{"id":"1rc1","registryId":"1r1"}`))
		case request.Method == http.MethodPut && request.URL.Path == "/v2-beta/registryCredentials/1rc1":
			var credential platformapi.RegistryCredential
			if err := json.NewDecoder(request.Body).Decode(&credential); err != nil {
				t.Error(err)
				http.Error(w, `{}`, http.StatusBadRequest)
				return
			}
			fixture.credentialUpdated++
			fixture.storedPassword = credential.SecretValue
			_, _ = w.Write([]byte(`{"id":"1rc1","registryId":"1r1"}`))
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.String())
			http.Error(w, `{}`, http.StatusNotFound)
		}
	})
}

func TestAWSAndPlatformLifecycleWithEndpointOverride(t *testing.T) {
	fixture := &synchronizationFixture{tokenPassword: "first-sensitive-password"}
	server := httptest.NewServer(fixture.handler(t))
	defer server.Close()

	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ECR_ENDPOINT_URL", server.URL)

	platformClient, err := platformapi.NewClient(server.URL+"/v1", "platform-access", "platform-secret")
	if err != nil {
		t.Fatal(err)
	}
	platform := &Platform{AutoCreate: true}
	policy := retryPolicy{
		maxAttempts: 2,
		delay:       func(int) time.Duration { return 0 },
		sleep:       func(time.Duration) {},
	}

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	ecrClient, err := awsClient()
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.updateEcrWithRetry(
		ecrClient,
		platformClient.Registries,
		platformClient.RegistryCredentials,
		policy,
	); err != nil {
		t.Fatal(err)
	}

	fixture.mu.Lock()
	if fixture.registryCreated != 1 || fixture.credentialCreated != 1 {
		t.Fatalf("create lifecycle = registry:%d credential:%d", fixture.registryCreated, fixture.credentialCreated)
	}
	if fixture.storedPassword != "first-sensitive-password" {
		t.Fatalf("first credential was not stored")
	}
	fixture.tokenPassword = "second-sensitive-password"
	fixture.mu.Unlock()

	ecrClient, err = awsClient()
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.updateEcrWithRetry(
		ecrClient,
		platformClient.Registries,
		platformClient.RegistryCredentials,
		policy,
	); err != nil {
		t.Fatal(err)
	}

	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.registryCreated != 1 || fixture.credentialCreated != 1 || fixture.credentialUpdated != 1 {
		t.Fatalf(
			"update lifecycle = registry-created:%d credential-created:%d credential-updated:%d",
			fixture.registryCreated,
			fixture.credentialCreated,
			fixture.credentialUpdated,
		)
	}
	if fixture.storedPassword != "second-sensitive-password" {
		t.Fatalf("updated credential was not stored")
	}
	for _, secret := range []string{
		"first-sensitive-password",
		"second-sensitive-password",
		"test-secret",
		"platform-secret",
	} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("secret leaked to logs: %s", secret)
		}
	}
}

func TestAWSClientRejectsUnsafeEndpoint(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	for _, endpoint := range []string{
		"file:///tmp/ecr",
		"http://user:password@example.test",
		"/relative",
	} {
		t.Run(fmt.Sprintf("%q", endpoint), func(t *testing.T) {
			t.Setenv("AWS_ECR_ENDPOINT_URL", endpoint)
			if _, err := awsClient(); err == nil {
				t.Fatalf("expected endpoint to be rejected: %s", endpoint)
			}
		})
	}
}
