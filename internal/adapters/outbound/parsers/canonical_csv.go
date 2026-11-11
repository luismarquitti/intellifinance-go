package parsers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

// CanonicalCSVParser strictly reads explicit typed cash magnitudes and provenance.
type CanonicalCSVParser struct{}

var canonicalMagnitude = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,2})?$`)

// NewCanonicalCSVParser constructs a stateless strict parser.
func NewCanonicalCSVParser() *CanonicalCSVParser { return &CanonicalCSVParser{} }

// ParseCanonical is strict and all-or-nothing. Unlike the legacy bank sniffer,
// Tipo explicitly determines the sign of the magnitude column.
func (p *CanonicalCSVParser) ParseCanonical(ctx context.Context, req ports.CanonicalImportRequest) ([]*domain.Transaction, error) {
	if len(req.Content) == 0 || len(req.Content) > 15*1024*1024 || !utf8.Valid(req.Content) || req.SourceName == "" || req.TenantID == uuid.Nil || req.UserID == uuid.Nil || req.AccountID == uuid.Nil {
		return nil, errors.New("explicit source, fixed identity and bounded UTF-8 content required")
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(req.Content, []byte(utf8BOM))))
	reader.FieldsPerRecord = 5
	header, err := reader.Read()
	if err != nil {
		return nil, errors.New("canonical CSV header required")
	}
	expected := []string{"ID", "Data", "Descrição", "Valor", "Tipo"}
	for i, h := range header {
		if h != expected[i] {
			return nil, errors.New("canonical CSV schema must be ID,Data,Descrição,Valor,Tipo")
		}
	}
	hash := sha256.Sum256(req.Content)
	rows := make([]*domain.Transaction, 0)
	seen := make(map[string]bool)
	for line := 2; ; line++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("canonical CSV malformed at record %d", line)
		}
		physicalLine, _ := reader.FieldPos(0)
		if row[0] == "" || row[0] != strings.TrimSpace(row[0]) || len(row[0]) > 120 || seen[row[0]] {
			return nil, fmt.Errorf("missing, invalid or repeated original ID at record %d", line)
		}
		seen[row[0]] = true
		if !canonicalMagnitude.MatchString(row[3]) || (row[4] != "Receita" && row[4] != "Despesa") {
			return nil, fmt.Errorf("explicit cash magnitude/type required at record %d", line)
		}
		date, err := time.ParseInLocation("02/01/2006", row[1], domain.BrazilLocation)
		if err != nil {
			return nil, fmt.Errorf("invalid canonical date at record %d", line)
		}
		amount, err := decimal.NewFromString(row[3])
		if err != nil {
			return nil, fmt.Errorf("invalid canonical amount at record %d", line)
		}
		kind := domain.TransactionTypeIncome
		if row[4] == "Despesa" {
			kind = domain.TransactionTypeExpense
			amount = amount.Neg()
		}
		rowJSON, err := json.Marshal(row)
		if err != nil {
			return nil, errors.New("cannot retain original row")
		}
		rowHash := sha256.Sum256(rowJSON)
		tx, err := domain.NewTransaction(domain.NewTransactionParams{TenantID: req.TenantID, CreatedByUserID: &req.UserID, AccountID: req.AccountID, Date: date, Amount: amount, Description: row[2], Type: kind, Source: domain.SourceTypeCSV, ExternalID: row[0], Metadata: map[string]any{"provenance": map[string]any{"source_name": req.SourceName, "source_sha256": hex.EncodeToString(hash[:]), "source_row": physicalLine, "original_id": row[0], "original_fields": row, "row_sha256": hex.EncodeToString(rowHash[:])}}})
		if err != nil {
			return nil, fmt.Errorf("invalid canonical transaction at record %d", line)
		}
		rows = append(rows, tx)
	}
	if len(rows) == 0 {
		return nil, errors.New("canonical source contains no records")
	}
	return rows, nil
}
