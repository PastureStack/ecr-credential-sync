package mocks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/stretchr/testify/mock"
)

// ECRAPI mocks only the operation used by the credential synchronizer. The
// former AWS SDK v1 mock duplicated the complete generated ECR API even though
// production called exactly one method.
type ECRAPI struct {
	mock.Mock
}

func (client *ECRAPI) GetAuthorizationToken(
	ctx context.Context,
	input *ecr.GetAuthorizationTokenInput,
	_ ...func(*ecr.Options),
) (*ecr.GetAuthorizationTokenOutput, error) {
	result := client.Called(ctx, input)

	var output *ecr.GetAuthorizationTokenOutput
	if factory, ok := result.Get(0).(func(context.Context, *ecr.GetAuthorizationTokenInput) *ecr.GetAuthorizationTokenOutput); ok {
		output = factory(ctx, input)
	} else if result.Get(0) != nil {
		output = result.Get(0).(*ecr.GetAuthorizationTokenOutput)
	}

	return output, result.Error(1)
}
