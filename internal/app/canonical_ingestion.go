package app

import (
	"context"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// CanonicalIngestionService retains original bytes before any ledger commit.
// Domain coverage, opening anchors and transfer classification must be reviewed
// separately; accepting a source is not a financial reconciliation certificate.
type CanonicalIngestionService struct {
	parser     ports.CanonicalStatementParser
	repository ports.CanonicalImportRepository
	archive    ports.SourceArchive
}

// NewCanonicalIngestionService wires strict parsing, original retention and atomic persistence.
func NewCanonicalIngestionService(parser ports.CanonicalStatementParser, repository ports.CanonicalImportRepository, archive ports.SourceArchive) *CanonicalIngestionService {
	return &CanonicalIngestionService{parser: parser, repository: repository, archive: archive}
}

// Ingest validates the entire source and retains it before committing any ledger rows.
func (s *CanonicalIngestionService) Ingest(ctx context.Context, req ports.CanonicalImportRequest) (*ports.CanonicalImportResult, error) {
	rows, err := s.parser.ParseCanonical(ctx, req)
	if err != nil {
		return nil, err
	}
	if _, err := s.archive.Retain(ctx, req.Content); err != nil {
		return nil, err
	}
	return s.repository.ImportCanonical(ctx, rows)
}
