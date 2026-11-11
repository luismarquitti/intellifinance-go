package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/stretchr/testify/require"
)

type canonicalRepo struct{ calls int }

func (r *canonicalRepo) ImportCanonical(_ context.Context, rows []*domain.Transaction) (*ports.CanonicalImportResult, error) {
	r.calls++
	return &ports.CanonicalImportResult{Parsed: len(rows), Inserted: len(rows)}, nil
}

type canonicalArchive struct {
	fail  bool
	calls int
}

func (a *canonicalArchive) Retain(context.Context, []byte) (string, error) {
	a.calls++
	if a.fail {
		return "", errors.New("synthetic archive failure")
	}
	return "retained", nil
}

func TestCanonicalIngestionRequiresRetainedOriginalBeforeCommit(t *testing.T) {
	req := ports.CanonicalImportRequest{TenantID: uuid.New(), UserID: uuid.New(), AccountID: uuid.New(), SourceName: "synthetic.csv", Content: []byte("ID,Data,Descrição,Valor,Tipo\nX,01/02/2026,Sintético,1.00,Despesa\n")}
	repo := &canonicalRepo{}
	archive := &canonicalArchive{fail: true}
	service := app.NewCanonicalIngestionService(parsers.NewCanonicalCSVParser(), repo, archive)
	_, err := service.Ingest(context.Background(), req)
	require.Error(t, err)
	require.Zero(t, repo.calls)
	archive.fail = false
	result, err := service.Ingest(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 1, result.Inserted)
	require.Equal(t, 1, repo.calls)
	req.Content = []byte("invalid source")
	_, err = service.Ingest(context.Background(), req)
	require.Error(t, err)
	require.Equal(t, 2, archive.calls, "invalid source must not be retained or persisted")
	require.Equal(t, 1, repo.calls)
}
