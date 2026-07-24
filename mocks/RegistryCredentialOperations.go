package mocks

import (
	"github.com/PastureStack/ecr-credential-sync/internal/platformapi"
	"github.com/stretchr/testify/mock"
)

type RegistryCredentialOperations struct{ mock.Mock }

func (m *RegistryCredentialOperations) List(options *platformapi.ListOptions) (*platformapi.RegistryCredentialCollection, error) {
	result := m.Called(options)
	var credentials *platformapi.RegistryCredentialCollection
	if result.Get(0) != nil {
		credentials = result.Get(0).(*platformapi.RegistryCredentialCollection)
	}
	return credentials, result.Error(1)
}

func (m *RegistryCredentialOperations) Create(credential *platformapi.RegistryCredential) (*platformapi.RegistryCredential, error) {
	result := m.Called(credential)
	var created *platformapi.RegistryCredential
	if result.Get(0) != nil {
		created = result.Get(0).(*platformapi.RegistryCredential)
	}
	return created, result.Error(1)
}

func (m *RegistryCredentialOperations) Update(existing, update *platformapi.RegistryCredential) (*platformapi.RegistryCredential, error) {
	result := m.Called(existing, update)
	var updated *platformapi.RegistryCredential
	if result.Get(0) != nil {
		updated = result.Get(0).(*platformapi.RegistryCredential)
	}
	return updated, result.Error(1)
}
