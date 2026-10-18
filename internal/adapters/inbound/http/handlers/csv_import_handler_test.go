package handlers_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCSVIngestionService struct {
	result *ports.CSVIngestionResult
	err    error
}

func (m *mockCSVIngestionService) IngestCSV(ctx context.Context, tenantID, userID, accountID uuid.UUID, r io.Reader) (*ports.CSVIngestionResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestCSVIngestHandler_ImportCSV(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	service := &mockCSVIngestionService{
		result: &ports.CSVIngestionResult{
			TotalParsed: 5,
			Inserted:    4,
			Duplicates:  1,
		},
	}
	handler := handlers.NewCSVIngestHandler(service)

	// Prepare multipart form body
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("account_id", accountID.String()))

	part, err := writer.CreateFormFile("file", "extrato.csv")
	require.NoError(t, err)
	_, err = part.Write([]byte("Data,Descricao,Valor\n2026-02-01,Padaria,-15.50\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/transactions/import/csv", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Inject context
	ctx := middleware.WithTenantID(req.Context(), tenantID)
	ctx = middleware.WithUserID(ctx, userID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ImportCSV(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"total_parsed":5`)
	assert.Contains(t, rr.Body.String(), `"inserted":4`)
	assert.Contains(t, rr.Body.String(), `"duplicates":1`)
}
