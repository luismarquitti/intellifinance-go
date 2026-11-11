package problem_test

import (
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReviewTransferProvenanceErrorsConflict(t *testing.T) {
	for _, err := range []error{domain.ErrImmutableImportedTransaction, domain.ErrReservedProvenance} {
		require.Equal(t, 409, problem.FromDomainError("/transactions", err).Status)
	}
}
