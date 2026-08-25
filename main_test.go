package main

// Modified by PastureStack contributors for independent maintenance and rebranding.

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PastureStack/ecr-credential-sync/internal/platformapi"
	"github.com/PastureStack/ecr-credential-sync/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

func TestMain_basic(t *testing.T) {
	platform := &Platform{}
	mockEcr := new(mocks.ECRAPI)
	mockRegistry := new(mocks.RegistryOperations)
	mockRegistryCredential := new(mocks.RegistryCredentialOperations)
	mockEcr.On("GetAuthorizationToken", mock.Anything, &ecr.GetAuthorizationTokenInput{}).Return(
		&ecr.GetAuthorizationTokenOutput{
			AuthorizationData: []types.AuthorizationData{
				{
					ProxyEndpoint:      aws.String("https://012345678910.dkr.ecr.us-east-1.amazonaws.com"),
					AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("mockUser:mockPassword"))),
				},
			},
		}, nil)

	mockRegistry.On("List", &platformapi.ListOptions{}).Return(
		&platformapi.RegistryCollection{
			Data: []platformapi.Registry{
				{
					Resource: platformapi.Resource{
						ID: "1r1",
					},
					ServerAddress: "012345678910.dkr.ecr.us-east-1.amazonaws.com",
				},
			},
		},
		nil,
	)

	credential := platformapi.RegistryCredential{
		Resource:   platformapi.Resource{ID: "1rc1"},
		RegistryID: "1r1",
	}
	mockRegistryCredential.On("List", &platformapi.ListOptions{
		Filters: map[string]interface{}{
			"registryId": "1r1",
		},
	}).Return(&platformapi.RegistryCredentialCollection{
		Data: []platformapi.RegistryCredential{credential},
	}, nil)
	mockRegistryCredential.On("Update", &credential, &platformapi.RegistryCredential{
		PublicValue: "mockUser",
		SecretValue: "mockPassword",
		Email:       defaultCredentialEmail,
	}).Return(&platformapi.RegistryCredential{}, nil)

	platform.updateEcr(mockEcr, mockRegistry, mockRegistryCredential)

	mockEcr.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
	mockRegistryCredential.AssertExpectations(t)
}

func TestProcessTokenRedactsMalformedAuthorizationToken(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	secretToken := "mockUserWithoutPasswordSecret"
	platform := &Platform{}
	platform.processToken(&types.AuthorizationData{
		ProxyEndpoint:      aws.String("https://012345678910.dkr.ecr.us-east-1.amazonaws.com"),
		AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte(secretToken))),
	}, nil, nil)

	output := logs.String()
	if strings.Contains(output, secretToken) {
		t.Fatalf("malformed authorization token leaked to logs: %s", output)
	}
	if !strings.Contains(output, "value redacted") {
		t.Fatalf("expected redaction marker in log output, got: %s", output)
	}
}

func TestProcessTokenRejectsCredentialBearingProxyEndpointWithoutLoggingIt(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	secret := "proxy-url-secret"
	platform := &Platform{}
	err := platform.processToken(&types.AuthorizationData{
		ProxyEndpoint:      aws.String("https://user:" + secret + "@registry.example.test"),
		AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("AWS:token"))),
	}, nil, nil)
	if err == nil {
		t.Fatal("expected credential-bearing proxy endpoint to be rejected")
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(err.Error(), secret) {
		t.Fatal("credential-bearing proxy endpoint leaked to logs or error text")
	}
}

func TestProcessTokenHandlesMissingAuthorizationFields(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	platform := &Platform{}
	platform.processToken(nil, nil, nil)
	platform.processToken(&types.AuthorizationData{
		ProxyEndpoint: aws.String("https://012345678910.dkr.ecr.us-east-1.amazonaws.com"),
	}, nil, nil)
	platform.processToken(&types.AuthorizationData{
		AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("mockUser:mockPassword"))),
	}, nil, nil)

	output := logs.String()
	for _, expected := range []string{
		"Authorization data is missing",
		"Authorization token is missing",
		"Proxy endpoint is missing",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in log output, got: %s", expected, output)
		}
	}
}

func TestMain_autoCreate(t *testing.T) {
	platform := &Platform{
		AutoCreate: true,
	}
	mockEcr := new(mocks.ECRAPI)
	mockRegistry := new(mocks.RegistryOperations)
	mockRegistryCredential := new(mocks.RegistryCredentialOperations)
	mockEcr.On("GetAuthorizationToken", mock.Anything, &ecr.GetAuthorizationTokenInput{}).Return(
		&ecr.GetAuthorizationTokenOutput{
			AuthorizationData: []types.AuthorizationData{
				{
					ProxyEndpoint:      aws.String("https://012345678910.dkr.ecr.us-east-1.amazonaws.com"),
					AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("mockUser:mockPassword"))),
				},
			},
		}, nil)

	mockRegistry.On("List", &platformapi.ListOptions{}).Return(
		&platformapi.RegistryCollection{
			Data: []platformapi.Registry{},
		},
		nil,
	)

	mockRegistry.On("Create",
		&platformapi.Registry{
			ServerAddress: "012345678910.dkr.ecr.us-east-1.amazonaws.com",
		},
	).Return(&platformapi.Registry{
		Resource: platformapi.Resource{
			ID: "1r1",
		},
		ServerAddress: "012345678910.dkr.ecr.us-east-1.amazonaws.com",
	}, nil)

	mockRegistryCredential.On("Create", &platformapi.RegistryCredential{
		RegistryID:  "1r1",
		PublicValue: "mockUser",
		SecretValue: "mockPassword",
		Email:       defaultCredentialEmail,
	}).Return(&platformapi.RegistryCredential{
		Resource:    platformapi.Resource{ID: "1rc1"},
		RegistryID:  "1r1",
		PublicValue: "mockUser",
		SecretValue: "mockPassword",
		Email:       defaultCredentialEmail,
	}, nil)

	platform.updateEcr(mockEcr, mockRegistry, mockRegistryCredential)

	mockEcr.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
	mockRegistryCredential.AssertExpectations(t)
}

func TestRetryPolicyStopsImmediatelyAfterSuccess(t *testing.T) {
	attempts := 0
	sleepCalls := 0
	policy := retryPolicy{
		maxAttempts: 3,
		delay: func(attempt int) time.Duration {
			return time.Duration(attempt) * time.Second
		},
		sleep: func(time.Duration) {
			sleepCalls++
		},
	}

	err := policy.do(func() error {
		attempts++
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected exactly one attempt, got %d", attempts)
	}
	if sleepCalls != 0 {
		t.Fatalf("successful operation must not sleep, got %d sleep calls", sleepCalls)
	}
}

func TestRetryPolicyUsesInjectedBackoff(t *testing.T) {
	attempts := 0
	var waits []time.Duration
	policy := retryPolicy{
		maxAttempts: 4,
		delay: func(attempt int) time.Duration {
			return time.Duration(attempt) * 10 * time.Millisecond
		},
		sleep: func(delay time.Duration) {
			waits = append(waits, delay)
		},
	}

	err := policy.do(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected three attempts, got %d", attempts)
	}
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	if !reflect.DeepEqual(waits, want) {
		t.Fatalf("unexpected injected waits: got %v, want %v", waits, want)
	}
}

func TestRetryPolicyReturnsLastErrorWithoutFinalSleep(t *testing.T) {
	attempts := 0
	sleepCalls := 0
	wantErr := errors.New("still unavailable")
	policy := retryPolicy{
		maxAttempts: 3,
		delay:       func(int) time.Duration { return time.Millisecond },
		sleep:       func(time.Duration) { sleepCalls++ },
	}

	err := policy.do(func() error {
		attempts++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected last error %v, got %v", wantErr, err)
	}
	if attempts != 3 {
		t.Fatalf("expected three attempts, got %d", attempts)
	}
	if sleepCalls != 2 {
		t.Fatalf("expected sleeps only between attempts, got %d", sleepCalls)
	}
}

func TestParseRegistryIDsNormalizesAndDeduplicates(t *testing.T) {
	got, err := parseRegistryIDs("123456789012, 210987654321,123456789012")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"123456789012", "210987654321"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registry IDs = %v, want %v", got, want)
	}
}

func TestParseRegistryIDsRejectsUnsafeValues(t *testing.T) {
	for _, input := range []string{
		"",
		"123",
		"12345678901x",
		"123456789012,not-an-account",
	} {
		if _, err := parseRegistryIDs(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}

func TestRequiredPlatformEnvironmentPrefersNeutralName(t *testing.T) {
	t.Setenv("PLATFORM_TEST_VALUE", "neutral")
	t.Setenv("CATTLE_TEST_VALUE", "compatibility")

	got, err := requiredPlatformEnvironment(
		"PLATFORM_TEST_VALUE", "CATTLE_TEST_VALUE")
	if err != nil {
		t.Fatal(err)
	}
	if got != "neutral" {
		t.Fatalf("value = %q, want neutral value", got)
	}
}

func TestRequiredPlatformEnvironmentUsesCompatibilityFallback(t *testing.T) {
	t.Setenv("PLATFORM_TEST_VALUE", "")
	t.Setenv("CATTLE_TEST_VALUE", "compatibility")

	got, err := requiredPlatformEnvironment(
		"PLATFORM_TEST_VALUE", "CATTLE_TEST_VALUE")
	if err != nil {
		t.Fatal(err)
	}
	if got != "compatibility" {
		t.Fatalf("value = %q, want compatibility value", got)
	}
}

func TestPingContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()
	ping(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "pong" {
		t.Fatalf("GET /ping = %d %q", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/ping", nil)
	response = httptest.NewRecorder()
	ping(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /ping status = %d", response.Code)
	}
}
