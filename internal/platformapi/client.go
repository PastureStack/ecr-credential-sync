package platformapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout  = 10 * time.Second
	maxResponseSize = 4 << 20
)

type ListOptions struct {
	Filters map[string]interface{}
}

type Resource struct {
	ID    string            `json:"id,omitempty"`
	Links map[string]string `json:"links,omitempty"`
}

type Registry struct {
	Resource
	ServerAddress string `json:"serverAddress,omitempty"`
}

type RegistryCollection struct {
	Data []Registry `json:"data,omitempty"`
}

type RegistryCredential struct {
	Resource
	Email       string `json:"email,omitempty"`
	PublicValue string `json:"publicValue,omitempty"`
	RegistryID  string `json:"registryId,omitempty"`
	SecretValue string `json:"secretValue,omitempty"`
}

type RegistryCredentialCollection struct {
	Data []RegistryCredential `json:"data,omitempty"`
}

type RegistryOperations interface {
	List(*ListOptions) (*RegistryCollection, error)
	Create(*Registry) (*Registry, error)
}

type RegistryCredentialOperations interface {
	List(*ListOptions) (*RegistryCredentialCollection, error)
	Create(*RegistryCredential) (*RegistryCredential, error)
	Update(*RegistryCredential, *RegistryCredential) (*RegistryCredential, error)
}

type Client struct {
	Registries          RegistryOperations
	RegistryCredentials RegistryCredentialOperations
	baseURL             string
	accessKey           string
	secretKey           string
	http                *http.Client
}

type registryService struct{ client *Client }
type registryCredentialService struct{ client *Client }

func NewClient(rawURL, accessKey, secretKey string) (*Client, error) {
	if strings.TrimSpace(accessKey) == "" {
		return nil, fmt.Errorf("platform access key is required")
	}
	if strings.TrimSpace(secretKey) == "" {
		return nil, fmt.Errorf("platform secret key is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse platform URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("platform URL must use http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("platform URL must include a host")
	}
	if u.User != nil {
		return nil, fmt.Errorf("platform URL must not contain user information")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("platform URL must not contain a query or fragment")
	}
	switch {
	case u.Path == "" || u.Path == "/":
		u.Path = "/v2-beta"
	case u.Path == "/v1":
		u.Path = "/v2-beta"
	case strings.HasPrefix(u.Path, "/v1/"):
		u.Path = "/v2-beta/" + strings.TrimPrefix(u.Path, "/v1/")
	}

	client := &Client{
		baseURL:   strings.TrimRight(u.String(), "/"),
		accessKey: accessKey,
		secretKey: secretKey,
		http: &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	client.Registries = &registryService{client: client}
	client.RegistryCredentials = &registryCredentialService{client: client}
	return client, nil
}

func (service *registryService) List(opts *ListOptions) (*RegistryCollection, error) {
	result := &RegistryCollection{}
	if err := service.client.list("registries", opts, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (service *registryService) Create(registry *Registry) (*Registry, error) {
	result := &Registry{}
	if err := service.client.write(http.MethodPost, service.client.baseURL+"/registries", registry, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (service *registryCredentialService) List(opts *ListOptions) (*RegistryCredentialCollection, error) {
	result := &RegistryCredentialCollection{}
	if err := service.client.list("registryCredentials", opts, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (service *registryCredentialService) Create(credential *RegistryCredential) (*RegistryCredential, error) {
	result := &RegistryCredential{}
	if err := service.client.write(http.MethodPost, service.client.baseURL+"/registryCredentials", credential, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (service *registryCredentialService) Update(existing, update *RegistryCredential) (*RegistryCredential, error) {
	if existing.ID == "" {
		return nil, fmt.Errorf("registry credential ID is required for update")
	}
	endpoint := service.client.baseURL + "/registryCredentials/" + url.PathEscape(existing.ID)
	result := &RegistryCredential{}
	if err := service.client.write(http.MethodPut, endpoint, update, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (client *Client) list(resource string, opts *ListOptions, result interface{}) error {
	endpoint, err := url.Parse(client.baseURL + "/" + resource)
	if err != nil {
		return err
	}
	if opts != nil {
		query := endpoint.Query()
		for key, value := range opts.Filters {
			if values, ok := value.([]string); ok {
				for _, item := range values {
					query.Add(key, item)
				}
				continue
			}
			query.Add(key, fmt.Sprint(value))
		}
		endpoint.RawQuery = query.Encode()
	}
	return client.request(http.MethodGet, endpoint.String(), nil, result)
}

func (client *Client) write(method, endpoint string, input, result interface{}) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode platform API request: %w", err)
	}
	return client.request(method, endpoint, bytes.NewReader(body), result)
}

func (client *Client) request(method, endpoint string, body io.Reader, result interface{}) error {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return fmt.Errorf("create platform API request: %w", err)
	}
	req.SetBasicAuth(client.accessKey, client.secretKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.http.Do(req)
	if err != nil {
		return fmt.Errorf("platform API request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return fmt.Errorf("read platform API response: %w", err)
	}
	if len(responseBody) > maxResponseSize {
		return fmt.Errorf("platform API response exceeds %d bytes", maxResponseSize)
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("platform API returned %s", resp.Status)
	}
	if result == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("decode platform API response: %w", err)
	}
	return nil
}
