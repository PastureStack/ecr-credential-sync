package mocks

import (
	"github.com/PastureStack/ecr-credential-sync/internal/platformapi"
	"github.com/stretchr/testify/mock"
)

type RegistryOperations struct{ mock.Mock }

func (m *RegistryOperations) List(options *platformapi.ListOptions) (*platformapi.RegistryCollection, error) {
	result := m.Called(options)
	var registries *platformapi.RegistryCollection
	if result.Get(0) != nil {
		registries = result.Get(0).(*platformapi.RegistryCollection)
	}
	return registries, result.Error(1)
}

func (m *RegistryOperations) Create(registry *platformapi.Registry) (*platformapi.Registry, error) {
	result := m.Called(registry)
	var created *platformapi.Registry
	if result.Get(0) != nil {
		created = result.Get(0).(*platformapi.Registry)
	}
	return created, result.Error(1)
}
