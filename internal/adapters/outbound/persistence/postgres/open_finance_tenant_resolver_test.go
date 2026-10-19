package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
)

func TestOpenFinanceTenantResolver_EmptyItemID(t *testing.T) {
	t.Parallel()

	resolver := postgres.NewOpenFinanceTenantResolver(nil)
	tenantID, err := resolver.ResolveTenantByItemID(context.Background(), "   ")

	if err == nil {
		t.Fatal("expected error for empty item ID, got nil")
	}
	if tenantID != uuid.Nil {
		t.Fatalf("expected uuid.Nil, got %s", tenantID)
	}
}
