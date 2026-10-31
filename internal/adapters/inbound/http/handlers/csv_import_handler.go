package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/dto"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// CSVIngestHandler handles synchronous CSV statement imports.
type CSVIngestHandler struct {
	csvService ports.CSVIngestionService
}

// NewCSVIngestHandler creates a new CSVIngestHandler instance.
func NewCSVIngestHandler(csvService ports.CSVIngestionService) *CSVIngestHandler {
	return &CSVIngestHandler{csvService: csvService}
}

// ImportCSV handles POST /transactions/import/csv.
func (h *CSVIngestHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	userID, _ := middleware.UserIDFromContext(r.Context())

	// Bound the entire request, not just the multipart in-memory buffer.
	r.Body = http.MaxBytesReader(w, r.Body, 15<<20)
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(1 << 20); err != nil { // #nosec G120 -- MaxBytesReader caps the body above; oversized-upload regression test verifies HTTP 413.
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			problem.Render(w, r, problem.New(http.StatusRequestEntityTooLarge, "about:blank",
				"Payload Too Large", "CSV upload exceeds the 15 MiB request limit.", r.URL.Path, "ERR_UPLOAD_TOO_LARGE"))
			return
		}
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Failed to parse multipart form."))
		return
	}

	accountIDStr := strings.TrimSpace(r.FormValue("account_id"))
	if accountIDStr == "" {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Missing required form field: 'account_id'."))
		return
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil || accountID == uuid.Nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Invalid 'account_id': must be a valid UUID."))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Missing uploaded file in field 'file'."))
		return
	}
	defer func() { _ = file.Close() }()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		problem.Render(w, r, problem.NewBadRequest(r.URL.Path, "Uploaded file must be a .csv file."))
		return
	}

	result, err := h.csvService.IngestCSV(r.Context(), tenantID, userID, accountID, file)
	if err != nil {
		problem.Render(w, r, problem.FromDomainError(r.URL.Path, err))
		return
	}

	resp := dto.CSVIngestResponse{
		TotalParsed: result.TotalParsed,
		Inserted:    result.Inserted,
		Duplicates:  result.Duplicates,
		Errors:      result.Errors,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
