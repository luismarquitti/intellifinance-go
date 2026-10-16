package parsers

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type fixtureTransaction struct {
	ID          string          `json:"id"`
	Date        time.Time       `json:"date"`
	Amount      decimal.Decimal `json:"amount"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Account     string          `json:"account"`
	Source      string          `json:"source"`
	FITID       string          `json:"fitid"`
}

func loadFixtureTransactions(t *testing.T) []fixtureTransaction {
	t.Helper()
	path := filepath.Clean(filepath.Join("..", "..", "..", "..", "test", "fixtures", "transacoes.json"))
	data, err := os.ReadFile(path) //nolint:gosec // synthetic test fixture path
	if err != nil {
		t.Fatalf("failed to read transacoes.json fixture: %v", err)
	}

	var txs []fixtureTransaction
	if unmarshalErr := json.Unmarshal(data, &txs); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal transacoes.json: %v", unmarshalErr)
	}
	if len(txs) != 100 {
		t.Fatalf("expected 100 synthetic transactions in transacoes.json, got %d", len(txs))
	}
	return txs
}

func openFixtureFile(t *testing.T, name string) *os.File {
	t.Helper()
	path := filepath.Clean(filepath.Join("..", "..", "..", "..", "test", "fixtures", name))
	f, err := os.Open(path) //nolint:gosec // synthetic test fixture path
	if err != nil {
		t.Fatalf("failed to open fixture %s: %v", name, err)
	}
	t.Cleanup(func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Errorf("failed to close fixture %s: %v", name, closeErr)
		}
	})
	return f
}

func TestOFXParser_SyntheticFixture(t *testing.T) {
	expectedTxs := loadFixtureTransactions(t)
	parser := NewOFXParser()

	if parser.Format() != ports.SourceFormatOFX {
		t.Fatalf("expected format %q, got %q", ports.SourceFormatOFX, parser.Format())
	}

	canByExt, err := parser.CanParse("extrato_itau.ofx", nil)
	if err != nil || !canByExt {
		t.Fatalf("expected CanParse to return true for .ofx filename, got %v, err=%v", canByExt, err)
	}

	canByStream, err := parser.CanParse("unknown.dat", strings.NewReader("OFXHEADER:100\nDATA:OFXSGML\n<OFX>"))
	if err != nil || !canByStream {
		t.Fatalf("expected CanParse to return true for OFX stream header, got %v, err=%v", canByStream, err)
	}

	f := openFixtureFile(t, "extrato_itau.ofx")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmt, err := parser.Parse(ctx, f, uuid.New())
	if err != nil {
		t.Fatalf("OFXParser.Parse failed on extrato_itau.ofx: %v", err)
	}

	if stmt.Format != ports.SourceFormatOFX {
		t.Errorf("expected statement format OFX, got %s", stmt.Format)
	}
	if stmt.BankID != "0341" {
		t.Errorf("expected BankID 0341, got %q", stmt.BankID)
	}
	if stmt.AccountID != "033123456" {
		t.Errorf("expected AccountID 033123456, got %q", stmt.AccountID)
	}
	if stmt.Currency != "BRL" {
		t.Errorf("expected Currency BRL, got %q", stmt.Currency)
	}
	if stmt.HeaderBalance == nil || !stmt.HeaderBalance.Equal(decimal.RequireFromString("15420.50")) {
		t.Errorf("expected HeaderBalance 15420.50, got %v", stmt.HeaderBalance)
	}
	if stmt.SkippedRows != 0 {
		t.Errorf("expected SkippedRows 0, got %d (warns: %v)", stmt.SkippedRows, stmt.ValidationWarns)
	}
	if len(stmt.Transactions) != len(expectedTxs) {
		t.Fatalf("expected %d transactions, got %d", len(expectedTxs), len(stmt.Transactions))
	}

	for i, exp := range expectedTxs {
		got := stmt.Transactions[i]
		if !got.Amount.Equal(exp.Amount) {
			t.Errorf("tx[%d] amount mismatch: got %s, want %s", i, got.Amount.String(), exp.Amount.String())
		}
		if got.Date.Format("2006-01-02") != exp.Date.Format("2006-01-02") {
			t.Errorf("tx[%d] date mismatch: got %s, want %s", i, got.Date.Format("2006-01-02"), exp.Date.Format("2006-01-02"))
		}
		if got.ExternalID != exp.FITID {
			t.Errorf("tx[%d] ExternalID mismatch: got %q, want %q", i, got.ExternalID, exp.FITID)
		}
		if got.Description != exp.Description {
			t.Errorf("tx[%d] Description mismatch: got %q, want %q", i, got.Description, exp.Description)
		}
		if got.CleanDescription != CleanMerchantName(exp.Description) {
			t.Errorf("tx[%d] CleanDescription mismatch: got %q, want %q", i, got.CleanDescription, CleanMerchantName(exp.Description))
		}
	}
}

func TestCSVParser_ItauSyntheticFixture(t *testing.T) {
	expectedTxs := loadFixtureTransactions(t)
	parser := NewCSVParser()

	if parser.Format() != ports.SourceFormatCSV {
		t.Fatalf("expected format %q, got %q", ports.SourceFormatCSV, parser.Format())
	}

	canByExt, err := parser.CanParse("extrato_itau.csv", nil)
	if err != nil || !canByExt {
		t.Fatalf("expected CanParse to return true for .csv filename, got %v, err=%v", canByExt, err)
	}

	f := openFixtureFile(t, "extrato_itau.csv")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defaultAcct := uuid.New()
	stmt, err := parser.Parse(ctx, f, defaultAcct)
	if err != nil {
		t.Fatalf("CSVParser.Parse failed on extrato_itau.csv: %v", err)
	}

	if stmt.Format != ports.SourceFormatCSV {
		t.Errorf("expected Format CSV, got %s", stmt.Format)
	}
	if stmt.AccountID != defaultAcct.String() {
		t.Errorf("expected AccountID %s, got %s", defaultAcct.String(), stmt.AccountID)
	}
	if stmt.SkippedRows != 0 {
		t.Errorf("expected SkippedRows 0, got %d (warns: %v)", stmt.SkippedRows, stmt.ValidationWarns)
	}
	if len(stmt.Transactions) != len(expectedTxs) {
		t.Fatalf("expected %d transactions, got %d", len(expectedTxs), len(stmt.Transactions))
	}

	for i, exp := range expectedTxs {
		got := stmt.Transactions[i]
		if !got.Amount.Equal(exp.Amount) {
			t.Errorf("tx[%d] amount mismatch: got %s, want %s", i, got.Amount.String(), exp.Amount.String())
		}
		if got.Date.Format("2006-01-02") != exp.Date.Format("2006-01-02") {
			t.Errorf("tx[%d] date mismatch: got %s, want %s", i, got.Date.Format("2006-01-02"), exp.Date.Format("2006-01-02"))
		}
		if got.ExternalID != exp.FITID {
			t.Errorf("tx[%d] ExternalID mismatch: got %q, want %q", i, got.ExternalID, exp.FITID)
		}
		if got.CategoryHint != exp.Category {
			t.Errorf("tx[%d] CategoryHint mismatch: got %q, want %q", i, got.CategoryHint, exp.Category)
		}
		if got.CleanDescription != CleanMerchantName(exp.Description) {
			t.Errorf("tx[%d] CleanDescription mismatch: got %q, want %q", i, got.CleanDescription, CleanMerchantName(exp.Description))
		}
	}
}

func TestCSVParser_NubankSyntheticFixture(t *testing.T) {
	expectedTxs := loadFixtureTransactions(t)
	parser := NewCSVParser()

	f := openFixtureFile(t, "fatura_nubank.csv")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmt, err := parser.Parse(ctx, f, uuid.New())
	if err != nil {
		t.Fatalf("CSVParser.Parse failed on fatura_nubank.csv: %v", err)
	}

	if stmt.SkippedRows != 0 {
		t.Errorf("expected SkippedRows 0, got %d (warns: %v)", stmt.SkippedRows, stmt.ValidationWarns)
	}
	if len(stmt.Transactions) != len(expectedTxs) {
		t.Fatalf("expected %d transactions, got %d", len(expectedTxs), len(stmt.Transactions))
	}

	for i, exp := range expectedTxs {
		got := stmt.Transactions[i]
		if !got.Amount.Equal(exp.Amount) {
			t.Errorf("tx[%d] inverted Nubank amount mismatch: got %s, want %s", i, got.Amount.String(), exp.Amount.String())
		}
		if got.Date.Format("2006-01-02") != exp.Date.Format("2006-01-02") {
			t.Errorf("tx[%d] date mismatch: got %s, want %s", i, got.Date.Format("2006-01-02"), exp.Date.Format("2006-01-02"))
		}
		if got.CategoryHint != strings.ToLower(exp.Category) {
			t.Errorf("tx[%d] CategoryHint mismatch: got %q, want %q", i, got.CategoryHint, strings.ToLower(exp.Category))
		}
		if got.Description != exp.Description {
			t.Errorf("tx[%d] Description mismatch: got %q, want %q", i, got.Description, exp.Description)
		}
	}
}

func TestExcelParser_AnnualSyntheticFixture(t *testing.T) {
	expectedTxs := loadFixtureTransactions(t)
	parser := NewExcelParser()

	if parser.Format() != ports.SourceFormatExcel {
		t.Fatalf("expected format %q, got %q", ports.SourceFormatExcel, parser.Format())
	}

	canByExt, err := parser.CanParse("historico_anual.xlsx", nil)
	if err != nil || !canByExt {
		t.Fatalf("expected CanParse true for .xlsx filename, got %v, err=%v", canByExt, err)
	}

	canByMagic, err := parser.CanParse("upload.bin", bytes.NewReader([]byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00}))
	if err != nil || !canByMagic {
		t.Fatalf("expected CanParse true for ZIP/OOXML magic header, got %v, err=%v", canByMagic, err)
	}

	f := openFixtureFile(t, "historico_anual.xlsx")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmt, err := parser.Parse(ctx, f, uuid.New())
	if err != nil {
		t.Fatalf("ExcelParser.Parse failed on historico_anual.xlsx: %v", err)
	}

	if stmt.Format != ports.SourceFormatExcel {
		t.Errorf("expected Format EXCEL, got %s", stmt.Format)
	}
	if stmt.SkippedRows != 0 {
		t.Errorf("expected SkippedRows 0, got %d (warns: %v)", stmt.SkippedRows, stmt.ValidationWarns)
	}
	if len(stmt.Transactions) != len(expectedTxs) {
		t.Fatalf("expected %d transactions, got %d", len(expectedTxs), len(stmt.Transactions))
	}

	for i, exp := range expectedTxs {
		got := stmt.Transactions[i]
		if !got.Amount.Equal(exp.Amount) {
			t.Errorf("tx[%d] amount mismatch: got %s, want %s", i, got.Amount.String(), exp.Amount.String())
		}
		if got.Date.Format("2006-01-02") != exp.Date.Format("2006-01-02") {
			t.Errorf("tx[%d] date mismatch: got %s, want %s", i, got.Date.Format("2006-01-02"), exp.Date.Format("2006-01-02"))
		}
		if got.CategoryHint != exp.Category {
			t.Errorf("tx[%d] CategoryHint mismatch: got %q, want %q", i, got.CategoryHint, exp.Category)
		}
		if got.AccountHint != exp.Account {
			t.Errorf("tx[%d] AccountHint mismatch: got %q, want %q", i, got.AccountHint, exp.Account)
		}
		if got.ExternalID != exp.FITID {
			t.Errorf("tx[%d] ExternalID mismatch: got %q, want %q", i, got.ExternalID, exp.FITID)
		}
	}
}

func TestOFXParser_Windows1252EncodingAndXML2(t *testing.T) {
	parser := NewOFXParser()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Real Windows-1252 / Latin-1 single-byte accented string:
	// "Transfer\xeancia Cart\xe3o Pagamento Eletr\xf4nico Servi\xe7os" -> "Transferência Cartão Pagamento Eletrônico Serviços"
	var cp1252Buf bytes.Buffer
	cp1252Buf.WriteString("OFXHEADER:100\nDATA:OFXSGML\nVERSION:102\nENCODING:USASCII\nCHARSET:1252\n\n<OFX>\n<BANKTRANLIST>\n")
	cp1252Buf.WriteString("<STMTTRN>\n<TRNTYPE>DEBIT\n<DTPOSTED>20250315\n<TRNAMT>-250.75\n<FITID>LATIN1-001\n<MEMO>")
	cp1252Buf.Write([]byte("Transfer\xeancia Cart\xe3o Pagamento Eletr\xf4nico Servi\xe7os\n"))
	// Second block: unclosed SGML followed by malformed block (missing TRNAMT) and valid block
	cp1252Buf.WriteString("<STMTTRN>\n<TRNTYPE>CREDIT\n<DTPOSTED>20250316\n<TRNAMT>1200.00\n<FITID>LATIN1-002\n<NAME>PADARIA S\xc3O JO\xc3O\n")
	cp1252Buf.WriteString("<STMTTRN>\n<TRNTYPE>DEBIT\n<DTPOSTED>20250317\n<FITID>MALFORMED-003\n<MEMO>MISSING AMOUNT\n")
	cp1252Buf.WriteString("</BANKTRANLIST>\n</OFX>")

	stmt, err := parser.Parse(ctx, &cp1252Buf, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error parsing CP1252 SGML OFX: %v", err)
	}
	if len(stmt.Transactions) != 2 {
		t.Fatalf("expected 2 valid transactions, got %d", len(stmt.Transactions))
	}
	if stmt.SkippedRows != 1 || len(stmt.ValidationWarns) != 1 {
		t.Errorf("expected 1 skipped malformed block, got SkippedRows=%d, Warns=%v", stmt.SkippedRows, stmt.ValidationWarns)
	}

	wantDesc := "Transferência Cartão Pagamento Eletrônico Serviços"
	if stmt.Transactions[0].Description != wantDesc {
		t.Errorf("expected decoded UTF-8 description %q, got %q", wantDesc, stmt.Transactions[0].Description)
	}
	if stmt.Transactions[1].Description != "PADARIA SÃO JOÃO" {
		t.Errorf("expected fallback NAME tag %q, got %q", "PADARIA SÃO JOÃO", stmt.Transactions[1].Description)
	}

	// 2. OFX 2.x XML format with closed tags
	xml2Payload := `<?xml version="1.0" encoding="UTF-8"?>
<?OFX OFXHEADER="200" VERSION="202" SECURITY="NONE" OLDFILEUID="NONE" NEWFILEUID="NONE"?>
<OFX>
  <BANKMSGSRSV1>
    <STMTRS>
      <CURDEF>BRL</CURDEF>
      <BANKACCTFROM><BANKID>0260</BANKID><ACCTID>998877-6</ACCTID></BANKACCTFROM>
      <BANKTRANLIST>
        <STMTTRN><TRNTYPE>DEBIT</TRNTYPE><DTPOSTED>20250810000000</DTPOSTED><TRNAMT>-89.90</TRNAMT><FITID>XML2-01</FITID><MEMO>COMPRA CARTAO - SPOTIFY BRASIL</MEMO></STMTTRN>
      </BANKTRANLIST>
    </STMTRS>
  </BANKMSGSRSV1>
</OFX>`

	xmlStmt, err := parser.Parse(ctx, strings.NewReader(xml2Payload), uuid.Nil)
	if err != nil {
		t.Fatalf("unexpected error parsing XML 2.x OFX: %v", err)
	}
	if xmlStmt.BankID != "0260" || xmlStmt.AccountID != "998877-6" {
		t.Errorf("unexpected bank/account metadata: %+v", xmlStmt)
	}
	if len(xmlStmt.Transactions) != 1 {
		t.Fatalf("expected 1 transaction in XML 2.x OFX, got %d", len(xmlStmt.Transactions))
	}
	if !xmlStmt.Transactions[0].Amount.Equal(decimal.RequireFromString("-89.90")) {
		t.Errorf("expected amount -89.90, got %s", xmlStmt.Transactions[0].Amount.String())
	}
	if xmlStmt.Transactions[0].CleanDescription != "SPOTIFY BRASIL" {
		t.Errorf("expected CleanDescription SPOTIFY BRASIL, got %q", xmlStmt.Transactions[0].CleanDescription)
	}
}

func TestCSVParser_EdgeCasesAndHeaderVariations(t *testing.T) {
	parser := NewCSVParser()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Tab-separated CSV with UTF-8 BOM, metadata preamble, and accounting currency formats
	tsvInput := "\xef\xbb\xbfExtrato Empresarial Mensal\n Emitido em: 01/05/2025\n\n" +
		"Dt. Lancamento\tHistórico\tValor (R$)\tDocumento\tCategoria\tConta\n" +
		"15-03-2025\tPIX ENVIADO - 15/03: FORNECEDOR ABC\t(1.234,56)\tDOC-01\tDespesas\t001-9\n" +
		"16/03/25\tPAGTO ELETRO - ENERGIA SUL\tR$ 450,25 D\tDOC-02\tUtilidades\t001-9\n" +
		"2025/03/17\tTED RECEBIDA - CLIENTE XYZ\tR$ +12,500.80\tDOC-03\tReceita\t001-9\n" +
		"invalid-date\tLINHA CORROMPIDA\t100,00\tDOC-04\tErro\t001-9\n" +
		"18/03/2025\tVALOR CORROMPIDO\tabc\tDOC-05\tErro\t001-9\n"

	canParse, err := parser.CanParse("unknown.txt", strings.NewReader(tsvInput))
	if err != nil || !canParse {
		t.Fatalf("expected CanParse to sniff TSV header with metadata preamble, got %v, err=%v", canParse, err)
	}

	stmt, err := parser.Parse(ctx, strings.NewReader(tsvInput), uuid.Nil)
	if err != nil {
		t.Fatalf("unexpected error parsing TSV statement: %v", err)
	}

	if stmt.SkippedRows != 2 {
		t.Errorf("expected 2 skipped rows, got %d (warns: %v)", stmt.SkippedRows, stmt.ValidationWarns)
	}
	if len(stmt.Transactions) != 3 {
		t.Fatalf("expected 3 valid transactions, got %d", len(stmt.Transactions))
	}

	wantAmounts := []string{"-1234.56", "-450.25", "12500.80"}
	wantCleanDescs := []string{"FORNECEDOR ABC", "ENERGIA SUL", "CLIENTE XYZ"}

	for i := range wantAmounts {
		if !stmt.Transactions[i].Amount.Equal(decimal.RequireFromString(wantAmounts[i])) {
			t.Errorf("tx[%d] amount = %s, want %s", i, stmt.Transactions[i].Amount.String(), wantAmounts[i])
		}
		if stmt.Transactions[i].CleanDescription != wantCleanDescs[i] {
			t.Errorf("tx[%d] clean description = %q, want %q", i, stmt.Transactions[i].CleanDescription, wantCleanDescs[i])
		}
	}

	// Missing mandatory column must return descriptive error
	_, err = parser.Parse(ctx, strings.NewReader("ColunaA;ColunaB\n1;2\n"), uuid.Nil)
	if err == nil {
		t.Fatal("expected error when mandatory CSV columns are missing, got nil")
	}
}

func TestExcelParser_MultiHeaderAndSummaryRows(t *testing.T) {
	f := excelize.NewFile()
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Errorf("failed to close excel file: %v", closeErr)
		}
	}()

	sheet := "Sheet1"
	rowsData := [][]string{
		{"Relatório Financeiro Consolidado"},
		{"Saldo Inicial", "R$ 10.000,00"},
		{},
		{"Data Lançamento", "Estabelecimento", "Valor", "Categoria", "Conta", "Identificador"},
		{"10/02/2025", "COMPRA COM CARTAO - MERCADO LIVRE 10/02", "-320,45", "Compras", "1234-5", "XLSX-001"},
		{"11/02/2025", "PIX RECEBIDO - REEMBOLSO", "1.500,00", "Renda", "1234-5", "XLSX-002"},
		{"Total do Mês", "", "1.179,55"},
	}

	for rIdx, row := range rowsData {
		for cIdx, val := range row {
			cell, cellErr := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
			if cellErr != nil {
				t.Fatalf("failed to compute cell name: %v", cellErr)
			}
			if setErr := f.SetCellValue(sheet, cell, val); setErr != nil {
				t.Fatalf("failed to set cell value: %v", setErr)
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("failed to write workbook to buffer: %v", err)
	}

	parser := NewExcelParser()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stmt, err := parser.Parse(ctx, &buf, uuid.Nil)
	if err != nil {
		t.Fatalf("ExcelParser.Parse failed: %v", err)
	}

	if len(stmt.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(stmt.Transactions))
	}
	if stmt.SkippedRows != 1 {
		t.Errorf("expected 1 skipped footer row, got %d", stmt.SkippedRows)
	}
	if !stmt.Transactions[0].Amount.Equal(decimal.RequireFromString("-320.45")) {
		t.Errorf("expected first amount -320.45, got %s", stmt.Transactions[0].Amount.String())
	}
	if stmt.Transactions[0].CleanDescription != "MERCADO LIVRE" {
		t.Errorf("expected CleanDescription MERCADO LIVRE, got %q", stmt.Transactions[0].CleanDescription)
	}
	if !stmt.Transactions[1].Amount.Equal(decimal.RequireFromString("1500.00")) {
		t.Errorf("expected second amount 1500.00, got %s", stmt.Transactions[1].Amount.String())
	}
}

func TestPDFParser_DigitalInvoiceExtraction(t *testing.T) {
	parser := NewPDFParser()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if parser.Format() != ports.SourceFormatPDF {
		t.Fatalf("expected format %q, got %q", ports.SourceFormatPDF, parser.Format())
	}

	canByExt, err := parser.CanParse("fatura_cartao.pdf", nil)
	if err != nil || !canByExt {
		t.Fatalf("expected CanParse true for .pdf filename, got %v, err=%v", canByExt, err)
	}

	// Build a synthetic digital PDF containing both an uncompressed stream and a zlib FlateDecode stream
	uncompressedStream := "BT\n/F1 12 Tf\n(21/04/2025 COMPRA CARTAO - SUPERMERCADO PAO DE ACUCAR -675,11) Tj\n[(10/06/2025 ) -120 (UBER DO BRASIL ) -80 (R$ -45,90)] TJ\nET"

	var compressedBuf bytes.Buffer
	zw := zlib.NewWriter(&compressedBuf)
	if _, writeErr := zw.Write([]byte("BT\n(09/06/2025 PIX RECEBIDO - CONSULTORIA 1.896,62) Tj\nET")); writeErr != nil {
		t.Fatalf("failed to write zlib stream: %v", writeErr)
	}
	if closeErr := zw.Close(); closeErr != nil {
		t.Fatalf("failed to close zlib writer: %v", closeErr)
	}

	var pdfDoc bytes.Buffer
	pdfDoc.WriteString("%PDF-1.4\n1 0 obj\n<< /Length 120 >>\nstream\n")
	pdfDoc.WriteString(uncompressedStream)
	pdfDoc.WriteString("\nendstream\nendobj\n2 0 obj\n<< /Filter /FlateDecode >>\nstream\n")
	pdfDoc.Write(compressedBuf.Bytes())
	pdfDoc.WriteString("\nendstream\nendobj\n%%EOF\n")

	canByHeader, err := parser.CanParse("statement.bin", bytes.NewReader(pdfDoc.Bytes()))
	if err != nil || !canByHeader {
		t.Fatalf("expected CanParse true for %%PDF- magic header, got %v, err=%v", canByHeader, err)
	}

	stmt, err := parser.Parse(ctx, bytes.NewReader(pdfDoc.Bytes()), uuid.Nil)
	if err != nil {
		t.Fatalf("PDFParser.Parse failed: %v", err)
	}

	if len(stmt.Transactions) != 3 {
		t.Fatalf("expected 3 extracted PDF transactions, got %d", len(stmt.Transactions))
	}

	expectedAmounts := []string{"-675.11", "-45.90", "1896.62"}
	expectedClean := []string{"SUPERMERCADO PAO DE ACUCAR", "UBER DO BRASIL", "CONSULTORIA"}

	for i := range expectedAmounts {
		if !stmt.Transactions[i].Amount.Equal(decimal.RequireFromString(expectedAmounts[i])) {
			t.Errorf("pdf tx[%d] amount = %s, want %s", i, stmt.Transactions[i].Amount.String(), expectedAmounts[i])
		}
		if stmt.Transactions[i].CleanDescription != expectedClean[i] {
			t.Errorf("pdf tx[%d] clean description = %q, want %q", i, stmt.Transactions[i].CleanDescription, expectedClean[i])
		}
	}

	// Corrupted / non-PDF stream must return an explicit error
	_, err = parser.Parse(ctx, strings.NewReader("NOT A PDF DOCUMENT"), uuid.Nil)
	if err == nil {
		t.Fatal("expected error for invalid PDF stream without %PDF- header, got nil")
	}
}

//nolint:misspell // Portuguese banking keywords in test cases
func TestCleanMerchantName(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"COMPRA CARTAO - SUPERMERCADO PAO DE ACUCAR 12/03", "SUPERMERCADO PAO DE ACUCAR"},
		{"Compra com Cartao: DROGARIA SAO PAULO 07/05/2025", "DROGARIA SAO PAULO"},
		{"PAGTO ELETRO - ENEL DISTRIBUICAO SP", "ENEL DISTRIBUICAO SP"},
		{"PIX TRANSF - RESTAURANTE MOCOTO 23/03", "RESTAURANTE MOCOTO"},
		{"Pix enviado - 25/12/2025: Restaurante Silva", "RESTAURANTE SILVA"},
		{"PIX RECEBIDO - CONSULTORIA", "CONSULTORIA"},
		{"TED ENVIADA - CONDOMINIO EDIFICIO REAL", "CONDOMINIO EDIFICIO REAL"},
		{"TED RECEBIDA - CLIENTE CONSULTORIA", "CLIENTE CONSULTORIA"},
		{"TED - SERVICOS CONTABEIS", "SERVICOS CONTABEIS"},
		{"DOC ELET TARIFA - BANCO ITAU", "BANCO ITAU"},
		{"DOC 987654", "987654"},
		{"PAGAMENTO BOLETO: CLARO BRASIL #9988", "CLARO BRASIL 9988"},
		{"Debito Automatico NETFLIX COM BR", "NETFLIX COM BR"},
		{"TRANSF ELETR TED - INVESTIMENTO", "INVESTIMENTO"},
		{"   PAG*CLINICAELUMA   ", "PAG CLINICAELUMA"},
	}

	for _, tc := range cases {
		got := CleanMerchantName(tc.raw)
		if got != tc.want {
			t.Errorf("CleanMerchantName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
		if cleanMerchantName(tc.raw) != tc.want {
			t.Errorf("cleanMerchantName(%q) = %q, want %q", tc.raw, cleanMerchantName(tc.raw), tc.want)
		}
		if CleanDescription(tc.raw) != tc.want {
			t.Errorf("CleanDescription(%q) = %q, want %q", tc.raw, CleanDescription(tc.raw), tc.want)
		}
	}
}

func TestParsers_ConcurrentExecutionRaceFree(t *testing.T) {
	ofxBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "test", "fixtures", "extrato_itau.ofx"))
	if err != nil {
		t.Fatalf("failed to load OFX fixture: %v", err)
	}
	csvBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "test", "fixtures", "extrato_itau.csv"))
	if err != nil {
		t.Fatalf("failed to load CSV fixture: %v", err)
	}
	xlsxBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "test", "fixtures", "historico_anual.xlsx"))
	if err != nil {
		t.Fatalf("failed to load XLSX fixture: %v", err)
	}
	pdfBytes := []byte("%PDF-1.4\nstream\nBT\n(21/04/2025 UBER DO BRASIL -35,90) Tj\nET\nendstream\n%%EOF")

	ofxParser := NewOFXParser()
	csvParser := NewCSVParser()
	excelParser := NewExcelParser()
	pdfParser := NewPDFParser()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	const workers = 32
	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}

			acctID := uuid.New()
			switch workerID % 4 {
			case 0:
				stmt, parseErr := ofxParser.Parse(ctx, bytes.NewReader(ofxBytes), acctID)
				if parseErr != nil {
					errCh <- fmt.Errorf("ofx worker %d: %w", workerID, parseErr)
					return
				}
				if len(stmt.Transactions) != 100 {
					errCh <- fmt.Errorf("ofx worker %d: got %d txs", workerID, len(stmt.Transactions))
				}
			case 1:
				stmt, parseErr := csvParser.Parse(ctx, bytes.NewReader(csvBytes), acctID)
				if parseErr != nil {
					errCh <- fmt.Errorf("csv worker %d: %w", workerID, parseErr)
					return
				}
				if len(stmt.Transactions) != 100 {
					errCh <- fmt.Errorf("csv worker %d: got %d txs", workerID, len(stmt.Transactions))
				}
			case 2:
				stmt, parseErr := excelParser.Parse(ctx, bytes.NewReader(xlsxBytes), acctID)
				if parseErr != nil {
					errCh <- fmt.Errorf("excel worker %d: %w", workerID, parseErr)
					return
				}
				if len(stmt.Transactions) != 100 {
					errCh <- fmt.Errorf("excel worker %d: got %d txs", workerID, len(stmt.Transactions))
				}
			case 3:
				stmt, parseErr := pdfParser.Parse(ctx, bytes.NewReader(pdfBytes), acctID)
				if parseErr != nil {
					errCh <- fmt.Errorf("pdf worker %d: %w", workerID, parseErr)
					return
				}
				if len(stmt.Transactions) != 1 {
					errCh <- fmt.Errorf("pdf worker %d: got %d txs", workerID, len(stmt.Transactions))
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for workerErr := range errCh {
		t.Errorf("concurrent worker failure: %v", workerErr)
	}
}

func TestParsers_ContextCancellationAndTimeout(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	parsersList := []ports.StatementParser{
		NewOFXParser(),
		NewCSVParser(),
		NewExcelParser(),
		NewPDFParser(),
	}

	for _, p := range parsersList {
		_, err := p.Parse(canceledCtx, strings.NewReader("dummy content"), uuid.Nil)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("parser %s: expected context.Canceled, got %v", p.Format(), err)
		}
	}
}
