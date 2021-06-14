package operation_manager

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	opstorage "github.com/topfreegames/maestro/internal/adapters/operation_storage/mock"
	"github.com/topfreegames/maestro/internal/core/entities/operation"
	porterrors "github.com/topfreegames/maestro/internal/core/ports/errors"
)

type testOperationDefinition struct{}

func (d *testOperationDefinition) ShouldExecute(currentOperations []operation.Operation) bool {
	return false
}
func (d *testOperationDefinition) Marshal() []byte            { return []byte{} }
func (d *testOperationDefinition) Unmarshal() ([]byte, error) { return []byte{}, nil }
func (d *testOperationDefinition) Name() string               { return "testOperationDefinition" }

type newOpMatcher struct {
	def operation.Definition
}

func (m *newOpMatcher) Matches(x interface{}) bool {
	op, _ := x.(*operation.Operation)
	_, err := uuid.Parse(op.ID)
	return err == nil && op.Status == operation.StatusPending && m.def.Name() == op.Definition.Name()
}

func (m *newOpMatcher) String() string {
	return fmt.Sprintf("a new operation with definition \"%s\"", m.def.Name())
}

func TestCreateOperation(t *testing.T) {
	cases := map[string]struct {
		definition operation.Definition
		storageErr error
	}{
		"create without errors": {
			definition: &testOperationDefinition{},
		},
		"create with storage errors": {
			definition: &testOperationDefinition{},
			storageErr: porterrors.ErrUnexpected,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			operationStorage := opstorage.NewMockOperationStorage(mockCtrl)
			opManager := New(operationStorage)

			ctx := context.Background()
			operationStorage.EXPECT().CreateOperation(ctx, &newOpMatcher{test.definition}).Return(test.storageErr)

			op, err := opManager.CreateOperation(ctx, test.definition)
			if test.storageErr != nil {
				require.ErrorIs(t, err, test.storageErr)
				require.Nil(t, op)
				return
			}

			require.NotNil(t, op)
			require.Equal(t, operation.StatusPending, op.Status)
		})
	}
}
