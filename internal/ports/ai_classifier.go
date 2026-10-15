package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// CategoryCandidate describes an available category option for AI classification.
type CategoryCandidate struct {
	ID         uuid.UUID              `json:"id"`
	Name       string                 `json:"name"`
	Type       domain.TransactionType `json:"type"`
	ParentName string                 `json:"parent_name,omitempty"`
}

// ClassificationRequest defines the input payload for classifying a single transaction.
type ClassificationRequest struct {
	TransactionID       uuid.UUID           `json:"transaction_id"`
	Description         string              `json:"description"`
	CleanDescription    string              `json:"clean_description"`
	Amount              decimal.Decimal     `json:"amount"`
	Date                time.Time           `json:"date"`
	AvailableCategories []CategoryCandidate `json:"available_categories"`
}

// ClassificationResult represents the AI model's categorical prediction and reasoning.
type ClassificationResult struct {
	TransactionID uuid.UUID       `json:"transaction_id"`
	CategoryID    uuid.UUID       `json:"category_id"`
	Confidence    decimal.Decimal `json:"confidence"` // 0.00 to 1.00
	Reasoning     string          `json:"reasoning,omitempty"`
	ModelVersion  string          `json:"model_version,omitempty"`
}

// AIClassifier defines the outbound port for AI/ML categorical inference engines (Vertex AI / LiteLLM).
type AIClassifier interface {
	// ClassifyBatch analyzes and predicts categories for a batch of transactions.
	ClassifyBatch(ctx context.Context, tenantID uuid.UUID, requests []ClassificationRequest) ([]ClassificationResult, error)

	// ClassifySingle analyzes and predicts the category for a single transaction.
	ClassifySingle(ctx context.Context, tenantID uuid.UUID, req ClassificationRequest) (*ClassificationResult, error)
}
