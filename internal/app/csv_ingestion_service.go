package app

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

var _ ports.CSVIngestionService = (*CSVIngestionService)(nil)

// CSVIngestionService synchronously processes uploaded CSV statement files.
type CSVIngestionService struct {
	parser  ports.StatementParser
	txRepo  ports.TransactionRepository
	accRepo ports.AccountRepository
}

// NewCSVIngestionService creates a new CSVIngestionService instance.
func NewCSVIngestionService(
	parser ports.StatementParser,
	txRepo ports.TransactionRepository,
	accRepo ports.AccountRepository,
) *CSVIngestionService {
	return &CSVIngestionService{
		parser:  parser,
		txRepo:  txRepo,
		accRepo: accRepo,
	}
}

// IngestCSV parses a CSV statement and saves non-duplicate transactions, updating the account balance.
func (s *CSVIngestionService) IngestCSV(
	ctx context.Context,
	tenantID, userID, accountID uuid.UUID,
	r io.Reader,
) (*ports.CSVIngestionResult, error) {
	if tenantID == uuid.Nil || accountID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}

	// 1. Verify that the account exists and belongs to the tenant
	acc, err := s.accRepo.GetByID(ctx, tenantID, accountID)
	if err != nil {
		return nil, fmt.Errorf("target account not found: %w", err)
	}

	// 2. Parse the CSV stream
	parsed, err := s.parser.Parse(ctx, r, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	result := &ports.CSVIngestionResult{
		TotalParsed: len(parsed.Transactions),
		Errors:      parsed.ValidationWarns,
	}

	if len(parsed.Transactions) == 0 {
		return result, nil
	}

	// 3. Build domain transactions and generate fingerprints
	var candidateTxs []*domain.Transaction
	var fingerprints []string

	for _, raw := range parsed.Transactions {
		if raw.Amount.IsZero() {
			result.Errors = append(result.Errors, fmt.Sprintf("skipping zero amount transaction: %s", raw.Description))
			continue
		}

		txType := domain.TransactionTypeExpense
		if raw.Amount.IsPositive() {
			txType = domain.TransactionTypeIncome
		}

		tx, err := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:        tenantID,
			AccountID:       acc.ID,
			CreatedByUserID: &userID,
			Date:            raw.Date,
			Amount:          raw.Amount,
			Description:     raw.Description,
			Type:            txType,
			Source:          domain.SourceTypeCSV,
			ExternalID:      raw.ExternalID,
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid row '%s': %v", raw.Description, err))
			continue
		}

		candidateTxs = append(candidateTxs, tx)
		fingerprints = append(fingerprints, tx.Fingerprint)
	}

	if len(candidateTxs) == 0 {
		return result, nil
	}

	// 4. Check existing fingerprints for deduplication
	existingMap, err := s.txRepo.FindExistingFingerprints(ctx, tenantID, fingerprints)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing fingerprints: %w", err)
	}

	var newTxs []*domain.Transaction
	balanceDelta := decimal.Zero

	for _, tx := range candidateTxs {
		if existingMap[tx.Fingerprint] {
			result.Duplicates++
			continue
		}

		newTxs = append(newTxs, tx)
		if tx.Status.AffectsBalance() {
			balanceDelta = balanceDelta.Add(tx.Amount)
		}
	}

	// 5. Persist new transactions in high-throughput batch
	if len(newTxs) > 0 {
		if err := s.txRepo.BatchCreate(ctx, newTxs); err != nil {
			return nil, fmt.Errorf("failed to persist batch transactions: %w", err)
		}
		result.Inserted = len(newTxs)

		// 6. Update account balance with aggregate delta
		if !balanceDelta.IsZero() {
			if err := s.accRepo.UpdateBalance(ctx, tenantID, accountID, balanceDelta); err != nil {
				return nil, fmt.Errorf("failed to adjust account balance: %w", err)
			}
		}
	}

	return result, nil
}
