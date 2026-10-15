package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// IngestionJobFilter defines querying criteria for ingestion jobs.
type IngestionJobFilter struct {
	AccountID *uuid.UUID
	Status    *domain.JobStatus
}

// IngestionJobRepository defines the persistence contract for IngestionJob tracking records.
type IngestionJobRepository interface {
	// Create persists a new ingestion job.
	Create(ctx context.Context, job *domain.IngestionJob) error

	// GetByID retrieves an ingestion job by ID within a tenant boundary.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.IngestionJob, error)

	// List queries ingestion jobs matching criteria within a tenant.
	List(ctx context.Context, tenantID uuid.UUID, filter IngestionJobFilter) ([]*domain.IngestionJob, error)

	// Update persists status, progress, and metrics updates for an ingestion job.
	Update(ctx context.Context, job *domain.IngestionJob) error
}
