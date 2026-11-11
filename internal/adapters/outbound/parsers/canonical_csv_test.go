package parsers_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

func TestCanonicalCSVPreservesMagnitudeSignAndOriginals(t *testing.T) {
	data := []byte("ID,Data,Descrição,Valor,Tipo\nSYN-1,01/02/2026,Compra sintética,10.50,Despesa\nSYN-2,01/02/2026,Compra sintética,10.50,Despesa\nSYN-3,02/02/2026,Crédito sintético,30.00,Receita\n")
	req := ports.CanonicalImportRequest{TenantID: uuid.New(), UserID: uuid.New(), AccountID: uuid.New(), SourceName: "synthetic.csv", Content: data}
	rows, err := parsers.NewCanonicalCSVParser().ParseCanonical(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Amount.String() != "-10.5" || rows[2].Amount.String() != "30" {
		t.Fatal("explicit source type must determine sign, without dropping legitimate repetition")
	}
	sum := sha256.Sum256(data)
	provenance := rows[0].Metadata["provenance"].(map[string]any)
	if provenance["source_sha256"] != hex.EncodeToString(sum[:]) || provenance["source_row"] != 2 || rows[0].ExternalID != "SYN-1" {
		t.Fatal("missing original provenance")
	}
}
