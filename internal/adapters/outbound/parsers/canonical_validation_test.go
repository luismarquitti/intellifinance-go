package parsers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

func TestCanonicalCSVRejectsWholeInvalidSource(t *testing.T) {
	header := "ID,Data,Descrição,Valor,Tipo\n"
	for _, row := range []string{
		"X,01/02/2026,Sintético,10.00,Previsão\n",
		"X,01/02/2026,Sintético,-10.00,Despesa\n",
		"X,01/02/2026,Sintético,10.001,Despesa\n",
		"X,01/02/2026,Sintético,1e2,Despesa\n",
		",01/02/2026,Sintético,10.00,Despesa\n",
		"X,01/02/2026,Sintético,10.00,Despesa\nX,01/02/2026,Sintético,10.00,Despesa\n",
		"X,31/02/2026,Sintético,10.00,Despesa\n",
		"X,01/02/2026,Sintético,10.00,Despesa,extra\n",
		"",
	} {
		t.Run(row, func(t *testing.T) {
			req := ports.CanonicalImportRequest{TenantID: uuid.New(), UserID: uuid.New(), AccountID: uuid.New(), SourceName: "synthetic.csv", Content: []byte(header + row)}
			if _, err := parsers.NewCanonicalCSVParser().ParseCanonical(context.Background(), req); err == nil {
				t.Fatal("invalid source accepted; no partial imports permitted")
			}
		})
	}
	req := ports.CanonicalImportRequest{TenantID: uuid.New(), UserID: uuid.New(), AccountID: uuid.New(), SourceName: "synthetic.csv", Content: []byte(header + strings.Repeat("x", 15*1024*1024))}
	if _, err := parsers.NewCanonicalCSVParser().ParseCanonical(context.Background(), req); err == nil {
		t.Fatal("oversized source accepted")
	}
}

func TestCanonicalCSVRetainsPhysicalLineForMultilineField(t *testing.T) {
	req := ports.CanonicalImportRequest{TenantID: uuid.New(), UserID: uuid.New(), AccountID: uuid.New(), SourceName: "synthetic.csv", Content: []byte("ID,Data,Descrição,Valor,Tipo\nX,01/02/2026,\"Sintético\nmultilinha\",10.00,Despesa\nY,02/02/2026,Outro,5.00,Despesa\n")}
	rows, err := parsers.NewCanonicalCSVParser().ParseCanonical(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if rows[1].Metadata["provenance"].(map[string]any)["source_row"] != 4 {
		t.Fatal("source row must be original physical line")
	}
}
