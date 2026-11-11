package ports

import (
	"context"

	"github.com/google/uuid"
)

// OpenFinanceTenantResolver defines the outbound port contract for resolving
// the tenant ID associated with an external Open Finance item ID.
type OpenFinanceTenantResolver interface {
	// ResolveTenantByItemID resolves the tenant ID belonging to the specified external item ID.
	// Returns an error if the item is not found or the resolution fails.
	ResolveTenantByItemID(ctx context.Context, itemID string) (uuid.UUID, error)
}
