package main

// Modified by PastureStack contributors for independent maintenance and rebranding.

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PastureStack/ecr-credential-sync/internal/platformapi"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	log "github.com/sirupsen/logrus"
)

const (
	defaultMaxECRAttempts  = 10
	defaultRetryDelay      = 10 * time.Second
	defaultCredentialEmail = "not-required@invalid"
)

var awsAccountIDPattern = regexp.MustCompile(`^[0-9]{12}$`)

// Platform holds the control-plane API configuration required by this service.
type Platform struct {
	URL         string
	AccessKey   string
	SecretKey   string
	RegistryIds []string
	AutoCreate  bool
	client      *platformapi.Client
}

// retryPolicy keeps retry timing injectable so tests never need to wait on the
// production backoff schedule.
type retryPolicy struct {
	maxAttempts int
	delay       func(attempt int) time.Duration
	sleep       func(time.Duration)
}

func defaultECRRetryPolicy() retryPolicy {
	return retryPolicy{
		maxAttempts: defaultMaxECRAttempts,
		delay: func(attempt int) time.Duration {
			return time.Duration(attempt) * defaultRetryDelay
		},
		sleep: time.Sleep,
	}
}

func (policy retryPolicy) do(operation func() error) error {
	maxAttempts := policy.maxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	delay := policy.delay
	if delay == nil {
		delay = func(int) time.Duration { return 0 }
	}

	sleep := policy.sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}
		if attempt == maxAttempts {
			return err
		}

		retryDelay := delay(attempt)
		log.Printf("ECR authorization attempt %d/%d failed: %s; retrying in %s", attempt, maxAttempts, err, retryDelay)
		if retryDelay > 0 {
			sleep(retryDelay)
		}
	}

	return err
}

func initLogger() error {
	// check if config param has been set for log level, otherwise the default of the logrus package will be used
	if logLevel, ok := os.LookupEnv("LOG_LEVEL"); ok && logLevel != "" {
		logLevelObj, err := log.ParseLevel(strings.ToLower(logLevel))
		if err != nil {
			return fmt.Errorf("parse LOG_LEVEL: %w", err)
		}
		log.SetLevel(logLevelObj)
	}
	// set log format to JSON
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	return nil
}

func main() {
	if err := initLogger(); err != nil {
		log.Fatal(err)
	}
	log.Info("Starting PastureStack ECR Credential Sync")

	platformURL, err := requiredEnvironment("PLATFORM_URL")
	if err != nil {
		log.Fatal(err)
	}
	platformAccessKey, err := requiredEnvironment("PLATFORM_ACCESS_KEY")
	if err != nil {
		log.Fatal(err)
	}
	platformSecretKey, err := requiredEnvironment("PLATFORM_SECRET_KEY")
	if err != nil {
		log.Fatal(err)
	}

	platform := Platform{
		URL:         platformURL,
		AccessKey:   platformAccessKey,
		SecretKey:   platformSecretKey,
		RegistryIds: []string{},
	}
	if val, ok := os.LookupEnv("AUTO_CREATE"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("Unable to parse boolean value from AUTO_CREATE: %s\n", err)
		}
		platform.AutoCreate = b
	}
	platformClient, err := platformapi.NewClient(platform.URL, platform.AccessKey, platform.SecretKey)
	if err != nil {
		log.Fatalf("Unable to create platform API client: %s\n", err)
	}
	platform.client = platformClient
	log.Debug("Created platform API client")

	if ids, ok := os.LookupEnv("AWS_ECR_REGISTRY_IDS"); ok && ids != "" {
		log.Debug("Detected AWS_ECR_REGISTRY_IDS config param")
		platform.RegistryIds, err = parseRegistryIDs(ids)
		if err != nil {
			log.Fatal(err)
		}
	}

	go func() {
		if err := healthcheck(); err != nil {
			log.Fatal("Health check listener failed: ", err)
		}
	}()

	synchronize := func() {
		client, err := awsClient()
		if err != nil {
			log.Error("Unable to create AWS ECR client: ", err)
			return
		}
		if err := platform.updateEcr(client, platform.client.Registries, platform.client.RegistryCredentials); err != nil {
			log.Error("ECR credential synchronization failed: ", err)
		}
	}

	synchronize()
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		log.Debug("Sleeping until next poll cycle")
		<-ticker.C
		synchronize()
	}
}

func requiredEnvironment(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func parseRegistryIDs(raw string) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		if !awsAccountIDPattern.MatchString(id) {
			return nil, fmt.Errorf("AWS_ECR_REGISTRY_IDS contains an invalid 12-digit account ID")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("AWS_ECR_REGISTRY_IDS must contain at least one 12-digit account ID")
	}
	return result, nil
}

func (platform *Platform) updateEcr(
	svc ecriface.ECRAPI,
	registryClient platformapi.RegistryOperations,
	registryCredentialClient platformapi.RegistryCredentialOperations) error {
	err := platform.updateEcrWithRetry(
		svc,
		registryClient,
		registryCredentialClient,
		defaultECRRetryPolicy(),
	)
	if err != nil {
		log.Errorln("Maximum retries for the AWS ECR API were reached. Last error:", err)
	}
	return err
}

func (platform *Platform) updateEcrWithRetry(
	svc ecriface.ECRAPI,
	registryClient platformapi.RegistryOperations,
	registryCredentialClient platformapi.RegistryCredentialOperations,
	policy retryPolicy) error {

	log.Println("Synchronizing ECR credentials")

	request := &ecr.GetAuthorizationTokenInput{}
	if len(platform.RegistryIds) > 0 {
		request = &ecr.GetAuthorizationTokenInput{RegistryIds: aws.StringSlice(platform.RegistryIds)}
	}
	return policy.do(func() error {
		log.Println("Attempting to call AWS API for ECR Authorization Token")
		resp, err := svc.GetAuthorizationToken(request)
		if err != nil {
			return fmt.Errorf("AWS ECR authorization token request failed: %w", err)
		}
		if resp == nil {
			return fmt.Errorf("AWS GetAuthorizationToken returned nil response")
		}
		log.Debugf("Returned from AWS GetAuthorizationToken call successfully with %d authorization data item(s)", len(resp.AuthorizationData))

		if len(resp.AuthorizationData) < 1 {
			return fmt.Errorf("AWS GetAuthorizationToken did not return authorization data")
		}

		var synchronizationErrors []error
		for _, data := range resp.AuthorizationData {
			if err := platform.processToken(data, registryClient, registryCredentialClient); err != nil {
				synchronizationErrors = append(synchronizationErrors, err)
			}
		}
		if err := errors.Join(synchronizationErrors...); err != nil {
			return err
		}
		return nil
	})
}

func (platform *Platform) processToken(
	data *ecr.AuthorizationData,
	registryClient platformapi.RegistryOperations,
	registryCredentialClient platformapi.RegistryCredentialOperations) error {

	if data == nil {
		log.Println("[missing-proxy-endpoint] Authorization data is missing")
		return fmt.Errorf("authorization data is missing")
	}
	if data.ProxyEndpoint == nil || *data.ProxyEndpoint == "" {
		log.Print("[missing-proxy-endpoint] Proxy endpoint is missing; skipping authorization token")
		return fmt.Errorf("proxy endpoint is missing")
	}

	registryURL, err := url.Parse(*data.ProxyEndpoint)
	if err != nil {
		return fmt.Errorf("parse ECR proxy endpoint: %w", err)
	}
	if registryURL.Scheme != "http" && registryURL.Scheme != "https" {
		return fmt.Errorf("ECR proxy endpoint must use http or https")
	}
	if registryURL.Host == "" || registryURL.User != nil || registryURL.RawQuery != "" || registryURL.Fragment != "" {
		return fmt.Errorf("ECR proxy endpoint must include a host and must not contain user information, a query, or a fragment")
	}
	ecrHost := registryURL.Host
	endpoint := ecrHost

	if data.AuthorizationToken == nil || *data.AuthorizationToken == "" {
		log.Printf("[%s] Authorization token is missing\n", endpoint)
		return fmt.Errorf("[%s] authorization token is missing", endpoint)
	}

	bytes, err := base64.StdEncoding.DecodeString(*data.AuthorizationToken)
	if err != nil {
		log.Printf("[%s] Error decoding authorization token: %s\n", endpoint, err)
		return fmt.Errorf("[%s] decode authorization token: %w", endpoint, err)
	}
	token := string(bytes[:len(bytes)])

	ecrUsername, ecrPassword, ok := strings.Cut(token, ":")
	if !ok || ecrUsername == "" || ecrPassword == "" {
		log.Printf("[%s] Authorization token does not contain usable <user>:<password> data; value redacted, decoded length=%d\n", endpoint, len(token))
		return fmt.Errorf("[%s] authorization token is malformed; value redacted", endpoint)
	}

	registries, err := registryClient.List(&platformapi.ListOptions{})
	if err != nil {
		log.Printf("[%s] Failed to retrieve registries: %s\n", endpoint, err)
		return fmt.Errorf("[%s] retrieve registries: %w", endpoint, err)
	}
	log.Printf("[%s] Looking for configured registry for host: %s\n", endpoint, ecrHost)
	for _, registry := range registries.Data {
		serverAddress, err := url.Parse(registry.ServerAddress)
		if err != nil {
			log.Printf("[%s] Failed to parse configured registry URL: %s\n", endpoint, registry.ServerAddress)
			continue
		}
		registryHost := serverAddress.Host
		if registryHost == "" {
			registryHost = serverAddress.Path
		}
		if registryHost == ecrHost {
			credentials, err := registryCredentialClient.List(&platformapi.ListOptions{
				Filters: map[string]interface{}{
					"registryId": registry.ID,
				},
			})
			if err != nil {
				log.Printf("[%s] Failed to retrieve registry credentials for ID %s: %s\n", endpoint, registry.ID, err)
				return fmt.Errorf("[%s] retrieve registry credentials for ID %s: %w", endpoint, registry.ID, err)
			}
			if len(credentials.Data) > 1 {
				return fmt.Errorf("[%s] registry %s has %d credential records; refusing an ambiguous update", endpoint, registry.ID, len(credentials.Data))
			}
			if len(credentials.Data) == 0 {
				if !platform.AutoCreate {
					return fmt.Errorf("[%s] registry %s has no credential record and AUTO_CREATE is disabled", endpoint, registry.ID)
				}
				_, err = registryCredentialClient.Create(&platformapi.RegistryCredential{
					RegistryID:  registry.ID,
					PublicValue: ecrUsername,
					SecretValue: ecrPassword,
					Email:       defaultCredentialEmail,
				})
				if err != nil {
					return fmt.Errorf("[%s] create credential for registry %s: %w", endpoint, registry.ID, err)
				}
				log.Printf("[%s] Successfully created a credential for registry %s\n", endpoint, registry.ID)
				return nil
			}
			credential := credentials.Data[0]
			_, err = registryCredentialClient.Update(&credential, &platformapi.RegistryCredential{
				PublicValue: ecrUsername,
				SecretValue: ecrPassword,
				Email:       defaultCredentialEmail,
			})
			if err != nil {
				log.Printf("[%s] Failed to update registry credential %s, %s\n", endpoint, credential.ID, err)
				return fmt.Errorf("[%s] update registry credential %s: %w", endpoint, credential.ID, err)
			} else {
				log.Printf("[%s] Successfully updated credentials %s for registry %s; registry address: %s\n", endpoint, credential.ID, registry.ID, registryHost)
			}
			return nil
		}
	}
	log.Printf("[%s] Did not find an existing registry for host: %s\n", endpoint, ecrHost)

	// No existing platform registry matched the ECR endpoint.
	if platform.AutoCreate {
		log.Printf("[%s] Automatically creating registry for host: %s\n", endpoint, ecrHost)
		registry, err := registryClient.Create(&platformapi.Registry{
			ServerAddress: ecrHost,
		})
		if err != nil {
			log.Printf("[%s] Error creating registry for host: %s, %s\n", endpoint, ecrHost, err)
			return fmt.Errorf("[%s] create registry for host %s: %w", endpoint, ecrHost, err)
		}
		if registry.ID == "" {
			return fmt.Errorf("[%s] created registry response did not include an ID", endpoint)
		}
		_, err = registryCredentialClient.Create(&platformapi.RegistryCredential{
			RegistryID:  registry.ID,
			PublicValue: ecrUsername,
			SecretValue: ecrPassword,
			Email:       defaultCredentialEmail,
		})
		if err != nil {
			log.Printf("[%s] Error creating registry credential for host: %s, %s\n", endpoint, ecrHost, err)
			return fmt.Errorf("[%s] create registry credential for host %s: %w", endpoint, ecrHost, err)
		}
		log.Printf("[%s] Successfully created registry %s and updated its credential\n", endpoint, registry.ID)
	} else {
		log.Printf("[%s] Failed to find a platform registry to update for ECR host: %s\n", endpoint, ecrHost)
		return fmt.Errorf("[%s] no matching registry and AUTO_CREATE is disabled", endpoint)
	}
	return nil
}

func healthcheck() error {
	listenPort := "8080"
	p, ok := os.LookupEnv("LISTEN_PORT")
	if ok {
		portNumber, err := strconv.Atoi(p)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return fmt.Errorf("LISTEN_PORT must be an integer from 1 through 65535")
		}
		listenPort = strconv.Itoa(portNumber)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", ping)
	log.Printf("Starting health check listener at :%s/ping\n", listenPort)
	server := &http.Server{
		Addr:              ":" + listenPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return server.ListenAndServe()
}

func ping(w http.ResponseWriter, r *http.Request) {
	log.Debug("Received health check request")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprint(w, "pong")
}

func awsClient() (*ecr.ECR, error) {
	region, err := requiredEnvironment("AWS_REGION")
	if err != nil {
		return nil, err
	}

	config := aws.NewConfig().WithRegion(region)
	if rawEndpoint := strings.TrimSpace(os.Getenv("AWS_ECR_ENDPOINT_URL")); rawEndpoint != "" {
		endpoint, err := url.Parse(rawEndpoint)
		if err != nil {
			return nil, fmt.Errorf("parse AWS_ECR_ENDPOINT_URL: %w", err)
		}
		if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
			return nil, fmt.Errorf("AWS_ECR_ENDPOINT_URL must use http or https")
		}
		if endpoint.Host == "" || endpoint.User != nil {
			return nil, fmt.Errorf("AWS_ECR_ENDPOINT_URL must include a host and must not contain user information")
		}
		config = config.
			WithEndpoint(strings.TrimRight(endpoint.String(), "/")).
			WithDisableSSL(endpoint.Scheme == "http")
	}

	awsSession := session.New(config)
	if roleArn := strings.TrimSpace(os.Getenv("AWS_ROLE_ARN")); roleArn != "" {
		log.Print("[awsClient] Assuming the configured AWS role")
		config = config.WithCredentials(stscreds.NewCredentials(awsSession, roleArn))
		awsSession = session.New(config)
	}
	return ecr.New(awsSession), nil
}
