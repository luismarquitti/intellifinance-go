# IntelliFinance Technical Specification: Ingestion Pipeline Adapter Contracts

- **Document ID**: SPEC-003-INGESTION
- **Status**: Authoritative / Production-Ready
- **Hexagonal Role**: Outbound Adapter (`internal/adapters/outbound/parsers`) & Application Domain Port
- **Supported Formats**: OFX (1.02 SGML & 2.x XML), CSV (Multi-Dialect Sniffing), Excel (XLSX via `excelize`), PDF (Digital Stream & Multimodal Gemini 2.0 OCR)
- **Deduplication Engine**: Deterministic SHA-256 Fingerprint Constraint

---

## 1. Pipeline Architecture Overview

The Ingestion Pipeline is responsible for consuming raw financial statements from varied bank formats, normalizing them into immutable domain value objects, evaluating deterministic cryptographic fingerprints to prevent duplicate ledger entries, and delegating newly discovered transactions to the AI categorization engine.

```
+-----------------------------------------------------------------------------------------------+
|                                      INGESTION PIPELINE                                       |
|                                                                                               |
|  [ File Upload / GCS ]                                                                        |
|           |                                                                                   |
|           v                                                                                   |
|  [ Ingestion Task Worker ]                                                                    |
|           |                                                                                   |
|           +---> [ Dialect & Format Sniffer ]                                                  |
|                       |                                                                       |
|        +--------------+--------------+---------------+----------------+                       |
|        |                             |               |                |                       |
|        v                             v               v                v                       |
|  [ OFX Parser ]              [ CSV Parser ]    [ Excel Parser ] [ PDF Engine ]                |
|  (SGML / XML Latin-1)       (BRL Dialects)    (excelize/v2)    (Digital / Gemini 2.0 OCR)     |
|        |                             |               |                |                       |
|        +--------------+--------------+---------------+----------------+                       |
|                       |                                                                       |
|                       v                                                                       |
|            [ Normalized Raw Transactions ]                                                    |
|                       |                                                                       |
|                       v                                                                       |
|            [ Deterministic SHA-256 Fingerprinter ]                                            |
|                       |                                                                       |
|                       v                                                                       |
|            [ Batch Deduplication Check (Database) ]                                           |
|                       |                                                                       |
|            +----------+----------+                                                            |
|            |                     |                                                            |
|    (New Transactions)     (Duplicates)                                                        |
|            |                     |                                                            |
|            v                     v                                                            |
|  [ AI Categorization ]    [ Count as Duplicate & Skip ]                                       |
|            |                                                                                  |
|            v                                                                                  |
|  [ Staging Bulk Copy (pgx.CopyFrom) & Atomic Upsert (INSERT ON CONFLICT) ]                     |
|            |                                                                                  |
|            v                                                                                  |
|  [ Update IngestionJob Metrics: total, inserted, duplicates ]                                 |
+-----------------------------------------------------------------------------------------------+
```

---

## 2. Core Go Interface Contracts (`internal/ports/parser.go`)

```go
package ports

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SourceFormat identifies the statement source format
type SourceFormat string

const (
	SourceFormatOFX   SourceFormat = "OFX"
	SourceFormatCSV   SourceFormat = "CSV"
	SourceFormatExcel SourceFormat = "EXCEL"
	SourceFormatPDF   SourceFormat = "PDF"
)

// RawTransaction represents an uncommitted parsed financial record
type RawTransaction struct {
	ExternalID       string          // Bank FITID or record identifier
	Date             time.Time       // Normalized transaction timestamp
	Amount           decimal.Decimal // Negative = Outflow/Expense, Positive = Inflow/Income
	Description      string          // Original raw bank memo
	CleanDescription string          // Normalized alphanumeric merchant name
	CategoryHint     string          // Bank-provided category hint if available
	AccountHint      string          // Bank account number if embedded in file
	Metadata         map[string]any  // Format-specific attributes (check number, doc number, etc.)
}

// ParsedStatement aggregates the results of a file parsing operation
type ParsedStatement struct {
	Format          SourceFormat
	AccountID       string
	BankID          string
	Currency        string
	PeriodStart     time.Time
	PeriodEnd       time.Time
	HeaderBalance   *decimal.Decimal
	Transactions    []RawTransaction
	SkippedRows     int
	ValidationWarns []string
}

// StatementParser defines the standard contract implemented by all format adapters
type StatementParser interface {
	CanParse(filename string, r io.Reader) (bool, error)
	Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ParsedStatement, error)
	Format() SourceFormat
}

// DeduplicationEngine calculates canonical fingerprints and rejects collisions
type DeduplicationEngine interface {
	ComputeFingerprint(tenantID, accountID uuid.UUID, date time.Time, amount decimal.Decimal, cleanDesc, externalID string) string
	FilterDuplicates(ctx context.Context, tenantID uuid.UUID, raw []RawTransaction) (unique []RawTransaction, duplicatesCount int, err error)
}
```

---

## 3. Format Parser Specifications

### 3.1 OFX Parser (Open Financial Exchange 1.02 & 2.x)

Brazilian banks (Itaú, Nubank, Banco do Brasil, Santander, Inter, Caixa) output OFX in two divergent variants:
- **OFX 1.02**: SGML-based syntax with unclosed tags (e.g. `<TRNAMT>-12.50\n<FITID>...`).
- **OFX 2.x**: Fully closed XML syntax with standard XML declarations.

#### Encoding & Character Set Handling
Brazilian OFX exports frequently declare `CHARSET:1252` or `ENCODING:USASCII` while embedding ISO-8859-1 (Latin-1) or Windows-1252 bytes. Accented characters ("Transferência", "Cartão", "Pagamento Eletrônico") must be decoded to UTF-8 without garbled replacement glyphs (`\ufffd`).

```go
package parsers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/charmap"
	"intellifinance/internal/ports"
)

type OFXParser struct{}

func NewOFXParser() *OFXParser {
	return &OFXParser{}
}

func (p *OFXParser) Format() ports.SourceFormat {
	return ports.SourceFormatOFX
}

func (p *OFXParser) CanParse(filename string, r io.Reader) (bool, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".ofx") {
		return true, nil
	}
	buf := make([]byte, 256)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}
	content := string(buf[:n])
	return strings.Contains(content, "OFXHEADER") || strings.Contains(content, "<OFX>"), nil
}

func (p *OFXParser) Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ports.ParsedStatement, error) {
	rawBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read ofx stream: %w", err)
	}

	// 1. Detect Encoding & Decode to UTF-8
	decodedText, err := p.decodeToUTF8(rawBytes)
	if err != nil {
		return nil, fmt.Errorf("ofx charset decoding failure: %w", err)
	}

	// 2. Extract Header Metadata (<BANKID>, <ACCTID>, <CURDEF>)
	stmt := &ports.ParsedStatement{
		Format:       ports.SourceFormatOFX,
		Currency:     "BRL",
		Transactions: make([]ports.RawTransaction, 0, 100),
	}
	p.extractHeaderMetadata(decodedText, stmt)

	// 3. Extract Transactions (<STMTTRN>...</STMTTRN>)
	trnRegex := regexp.MustCompile(`(?s)<STMTTRN>(.*?)</STMTTRN>`)
	matches := trnRegex.FindAllStringSubmatch(decodedText, -1)

	// Fallback for SGML without closing tags
	if len(matches) == 0 {
		matches = p.extractSGMLTransactions(decodedText)
	}

	for _, m := range matches {
		block := m[1]
		rawTx, err := p.parseTransactionBlock(block)
		if err != nil {
			stmt.ValidationWarns = append(stmt.ValidationWarns, err.Error())
			continue
		}
		stmt.Transactions = append(stmt.Transactions, rawTx)
	}

	return stmt, nil
}

func (p *OFXParser) decodeToUTF8(raw []byte) (string, error) {
	headerSlice := raw[:min(1024, len(raw))]
	isLatin1 := bytes.Contains(bytes.ToUpper(headerSlice), []byte("CHARSET:1252")) ||
		bytes.Contains(bytes.ToUpper(headerSlice), []byte("ENCODING:USASCII")) ||
		bytes.Contains(bytes.ToUpper(headerSlice), []byte("CHARSET:ISO-8859-1"))

	if isLatin1 {
		reader := charmap.Windows1252.NewDecoder().Reader(bytes.NewReader(raw))
		decoded, err := io.ReadAll(reader)
		if err == nil {
			return string(decoded), nil
		}
	}
	return string(raw), nil
}

func (p *OFXParser) extractHeaderMetadata(text string, stmt *ports.ParsedStatement) {
	bankIDReg := regexp.MustCompile(`<BANKID>([^\s<]+)`)
	if m := bankIDReg.FindStringSubmatch(text); len(m) > 1 {
		stmt.BankID = strings.TrimSpace(m[1])
	}
	acctIDReg := regexp.MustCompile(`<ACCTID>([^\s<]+)`)
	if m := acctIDReg.FindStringSubmatch(text); len(m) > 1 {
		stmt.AccountID = strings.TrimSpace(m[1])
	}
	curReg := regexp.MustCompile(`<CURDEF>([^\s<]+)`)
	if m := curReg.FindStringSubmatch(text); len(m) > 1 {
		stmt.Currency = strings.TrimSpace(m[1])
	}
}

func (p *OFXParser) extractSGMLTransactions(text string) [][]string {
	var result [][]string
	scanner := bufio.NewScanner(strings.NewReader(text))
	var currentBlock strings.Builder
	inBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "<STMTTRN>") {
			inBlock = true
			currentBlock.Reset()
			currentBlock.WriteString(line + "\n")
			continue
		}
		if inBlock {
			if strings.HasPrefix(line, "</STMTTRN>") || strings.HasPrefix(line, "<STMTTRN>") {
				result = append(result, []string{"", currentBlock.String()})
				currentBlock.Reset()
				if strings.HasPrefix(line, "<STMTTRN>") {
					currentBlock.WriteString(line + "\n")
				} else {
					inBlock = false
				}
				continue
			}
			currentBlock.WriteString(line + "\n")
		}
	}
	if inBlock && currentBlock.Len() > 0 {
		result = append(result, []string{"", currentBlock.String()})
	}
	return result
}

func (p *OFXParser) parseTransactionBlock(block string) (ports.RawTransaction, error) {
	tagValue := func(tag string) string {
		re := regexp.MustCompile(fmt.Sprintf(`<%s>([^<\r\n]+)`, tag))
		m := re.FindStringSubmatch(block)
		if len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
		return ""
	}

	rawAmt := tagValue("TRNAMT")
	if rawAmt == "" {
		return ports.RawTransaction{}, fmt.Errorf("missing TRNAMT in block: %s", block)
	}
	amount, err := decimal.NewFromString(rawAmt)
	if err != nil {
		return ports.RawTransaction{}, fmt.Errorf("invalid decimal TRNAMT '%s': %w", rawAmt, err)
	}

	rawDate := tagValue("DTPOSTED")
	if len(rawDate) < 8 {
		return ports.RawTransaction{}, fmt.Errorf("invalid or missing DTPOSTED '%s'", rawDate)
	}
	parsedDate, err := time.Parse("20060102", rawDate[:8])
	if err != nil {
		return ports.RawTransaction{}, fmt.Errorf("failed to parse DTPOSTED date '%s': %w", rawDate, err)
	}

	memo := tagValue("MEMO")
	if memo == "" {
		memo = tagValue("NAME")
	}

	return ports.RawTransaction{
		ExternalID:       tagValue("FITID"),
		Date:             parsedDate,
		Amount:           amount,
		Description:      memo,
		CleanDescription: CleanDescription(memo),
		Metadata: map[string]any{
			"check_num": tagValue("CHECKNUM"),
			"ref_num":   tagValue("REFNUM"),
		},
	}, nil
}
```

---

### 3.2 CSV Parser with Auto-Dialect Sniffing

Brazilian financial CSV exports differ significantly across banking institutions:
- **Separators**: Itaú and Inter use `;`, Nubank uses `,`, some corporate accounts use `\t`.
- **Number Formats**: Standard Brazilian formatting uses `.` for thousands and `,` for decimals (`-1.250,50`). Values may be wrapped in parentheses `(150,00)` to denote debits or include `"R$"` symbols.
- **Inverted Sign Logic**: Nubank credit card bill exports record card purchases as positive numbers and payments as negative numbers. IntelliFinance normalizes all expenses to negative amounts and income to positive amounts.

```go
package parsers

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"intellifinance/internal/ports"
)

type CSVParser struct{}

func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

func (p *CSVParser) Format() ports.SourceFormat {
	return ports.SourceFormatCSV
}

func (p *CSVParser) CanParse(filename string, r io.Reader) (bool, error) {
	return strings.HasSuffix(strings.ToLower(filename), ".csv"), nil
}

func (p *CSVParser) Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ports.ParsedStatement, error) {
	br := bufio.NewReader(r)
	sampleBytes, err := br.Peek(2048)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to peek csv header: %w", err)
	}

	// 1. Detect Delimiter
	delimiter := p.detectDelimiter(string(sampleBytes))

	csvReader := csv.NewReader(br)
	csvReader.Comma = delimiter
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv read failed: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv file contains insufficient rows")
	}

	// 2. Identify Column Indices from Header
	colMap, isNubankCredit := p.identifyColumns(rows[0])
	if colMap.dateIdx == -1 || colMap.amountIdx == -1 || colMap.descIdx == -1 {
		return nil, fmt.Errorf("unable to map mandatory csv columns (date, amount, description)")
	}

	stmt := &ports.ParsedStatement{
		Format:       ports.SourceFormatCSV,
		Currency:     "BRL",
		Transactions: make([]ports.RawTransaction, 0, len(rows)-1),
	}

	for lineNum, row := range rows[1:] {
		if len(row) <= max(colMap.dateIdx, max(colMap.amountIdx, colMap.descIdx)) {
			stmt.SkippedRows++
			continue
		}

		parsedDate, err := p.parseBrazilianDate(row[colMap.dateIdx])
		if err != nil {
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("line %d: invalid date '%s'", lineNum+2, row[colMap.dateIdx]))
			continue
		}

		amount, err := p.parseBrazilianAmount(row[colMap.amountIdx])
		if err != nil {
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("line %d: invalid amount '%s'", lineNum+2, row[colMap.amountIdx]))
			continue
		}

		// Adjust inverted Nubank credit card bill sign
		if isNubankCredit {
			amount = amount.Neg()
		}

		desc := strings.TrimSpace(row[colMap.descIdx])

		extID := ""
		if colMap.idIdx != -1 && len(row) > colMap.idIdx {
			extID = strings.TrimSpace(row[colMap.idIdx])
		}

		stmt.Transactions = append(stmt.Transactions, ports.RawTransaction{
			ExternalID:       extID,
			Date:             parsedDate,
			Amount:           amount,
			Description:      desc,
			CleanDescription: CleanDescription(desc),
		})
	}

	return stmt, nil
}

type columnMapping struct {
	dateIdx   int
	amountIdx int
	descIdx   int
	idIdx     int
}

func (p *CSVParser) detectDelimiter(sample string) rune {
	commaCount := strings.Count(sample, ",")
	semicolonCount := strings.Count(sample, ";")
	tabCount := strings.Count(sample, "\t")

	if semicolonCount > commaCount && semicolonCount > tabCount {
		return ';'
	}
	if tabCount > commaCount && tabCount > semicolonCount {
		return '\t'
	}
	return ','
}

func (p *CSVParser) identifyColumns(header []string) (columnMapping, bool) {
	mapping := columnMapping{dateIdx: -1, amountIdx: -1, descIdx: -1, idIdx: -1}
	isNubankCredit := false

	for i, col := range header {
		clean := strings.ToLower(strings.TrimSpace(col))
		switch clean {
		case "data", "date", "data lançamento", "dt. lancamento":
			mapping.dateIdx = i
		case "valor", "amount", "valor (r$)", "valor r$":
			mapping.amountIdx = i
		case "descrição", "descricao", "description", "histórico", "historico", "detalhes", "lançamento", "title":
			mapping.descIdx = i
		case "identificador", "id", "documento", "doc", "fitid":
			mapping.idIdx = i
		}
	}

	// Check if this matches Nubank credit card bill header: [date, category, title, amount]
	if len(header) == 4 && strings.ToLower(header[0]) == "date" && strings.ToLower(header[2]) == "title" {
		isNubankCredit = true
	}

	return mapping, isNubankCredit
}

func (p *CSVParser) parseBrazilianDate(val string) (time.Time, error) {
	clean := strings.TrimSpace(val)
	formats := []string{
		"02/01/2006",
		"2006-01-02",
		"02-01-2006",
		"02/01/06",
		"2006/01/02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, clean); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown date format '%s'", val)
}

func (p *CSVParser) parseBrazilianAmount(val string) (decimal.Decimal, error) {
	clean := strings.TrimSpace(val)
	clean = strings.ReplaceAll(clean, "R$", "")
	clean = strings.ReplaceAll(clean, " ", "")
	
	isNegative := false
	if strings.HasPrefix(clean, "(") && strings.HasSuffix(clean, ")") {
		isNegative = true
		clean = strings.Trim(clean, "()")
	} else if strings.HasPrefix(clean, "-") {
		isNegative = true
		clean = strings.TrimPrefix(clean, "-")
	}

	// Normalize Brazilian 1.250,50 -> 1250.50
	if strings.Contains(clean, ",") && strings.Contains(clean, ".") {
		clean = strings.ReplaceAll(clean, ".", "")
		clean = strings.ReplaceAll(clean, ",", ".")
	} else if strings.Contains(clean, ",") {
		clean = strings.ReplaceAll(clean, ",", ".")
	}

	dec, err := decimal.NewFromString(clean)
	if err != nil {
		return decimal.Zero, err
	}
	if isNegative {
		dec = dec.Neg()
	}
	return dec, nil
}
```

---

### 3.3 Excel (XLSX) Parser via `excelize/v2`

The Excel parser handles spreadsheets with multi-sheet structures and nested header summaries using low-memory streaming (`Rows()` iterator):

```go
package parsers

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"intellifinance/internal/ports"
)

type ExcelParser struct {
	csvFallback *CSVParser
}

func NewExcelParser() *ExcelParser {
	return &ExcelParser{csvFallback: NewCSVParser()}
}

func (p *ExcelParser) Format() ports.SourceFormat {
	return ports.SourceFormatExcel
}

func (p *ExcelParser) CanParse(filename string, r io.Reader) (bool, error) {
	return strings.HasSuffix(strings.ToLower(filename), ".xlsx"), nil
}

func (p *ExcelParser) Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ports.ParsedStatement, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("excel workbook contains no sheets")
	}

	// Read primary transaction sheet (default to active sheet or first)
	sheetName := sheets[0]
	rows, err := f.Rows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows in sheet '%s': %w", sheetName, err)
	}
	defer rows.Close()

	stmt := &ports.ParsedStatement{
		Format:       ports.SourceFormatExcel,
		Currency:     "BRL",
		Transactions: make([]ports.RawTransaction, 0, 200),
	}

	var headerRow []string
	var colMap columnMapping
	isHeaderIdentified := false

	for rows.Next() {
		row, err := rows.Columns()
		if err != nil || len(row) == 0 {
			continue
		}

		if !isHeaderIdentified {
			// Scan for recognizable header row
			for _, cell := range row {
				cleanCell := strings.ToLower(strings.TrimSpace(cell))
				if cleanCell == "data" || cleanCell == "date" || cleanCell == "lançamento" {
					headerRow = row
					colMap, _ = p.csvFallback.identifyColumns(headerRow)
					if colMap.dateIdx != -1 && colMap.amountIdx != -1 {
						isHeaderIdentified = true
					}
					break
				}
			}
			continue
		}

		if len(row) <= max(colMap.dateIdx, max(colMap.amountIdx, colMap.descIdx)) {
			stmt.SkippedRows++
			continue
		}

		parsedDate, err := p.csvFallback.parseBrazilianDate(row[colMap.dateIdx])
		if err != nil {
			continue
		}
		amount, err := p.csvFallback.parseBrazilianAmount(row[colMap.amountIdx])
		if err != nil {
			continue
		}

		desc := strings.TrimSpace(row[colMap.descIdx])
		stmt.Transactions = append(stmt.Transactions, ports.RawTransaction{
			Date:             parsedDate,
			Amount:           amount,
			Description:      desc,
			CleanDescription: CleanDescription(desc),
		})
	}

	return stmt, nil
}
```

---

### 3.4 Multimodal PDF Statement & Holerite Ingestion Engine

When processing digital or scanned bank statements and salary receipts (Holerites), text scrapers (`pdfcpu`, `pdf-parse`) frequently mangle complex tables and column headers. The IntelliFinance engine adopts a dual-tier processing architecture:

1. **Digital Stream Scan**: Rapidly inspects the PDF for embedded text vectors using `pdfcpu`. If a clean text table is verified, it parses via structured heuristics.
2. **Vertex AI Gemini 2.0 Multimodal OCR (Production)**: Transmits PDF binary bytes directly to Gemini 2.0 Flash (`google.golang.org/genai`). Gemini 2.0 inspects visual table lines, headers, bank logos, discounts (INSS, IRRF), and returns strict typed JSON.
3. **Local Homelab Fallback**: When operating offline or without GCP credentials, the adapter transparently targets LiteLLM proxy (`http://192.168.3.10:4000/v1/chat/completions`) or Ollama (`llama3.2-vision`).

#### Gemini 2.0 Multimodal Extraction JSON Schema

```json
{
  "account_identifier": "string",
  "bank_name": "string",
  "statement_period": {
    "start_date": "YYYY-MM-DD",
    "end_date": "YYYY-MM-DD"
  },
  "initial_balance": -100.50,
  "final_balance": 1540.20,
  "transactions": [
    {
      "date": "YYYY-MM-DD",
      "description": "string",
      "amount": -142.50,
      "category_hint": "string",
      "payment_method": "PIX"
    }
  ],
  "holerite_breakdown": {
    "is_holerite": true,
    "gross_salary": 15000.00,
    "inss_discount": 908.85,
    "irrf_discount": 2840.10,
    "other_discounts": 150.00,
    "net_salary": 11101.05
  }
}
```

---

## 4. Deterministic Deduplication Fingerprint Specification

### 4.1 Canonical Fingerprint Formula & Mathematical Formulation

To prevent duplicate transaction insertions across repeated statement uploads, overlapping date ranges, or repeated webhook deliveries, the system computes a deterministic, collision-free SHA-256 fingerprint:

$$\text{Fingerprint} = \text{SHA256}(\text{TenantID} \parallel \text{"\|"} \parallel \text{AccountID} \parallel \text{"\|"} \parallel \text{Date} \parallel \text{"\|"} \parallel \text{AmountCents} \parallel \text{"\|"} \parallel \text{CleanDescription} \parallel \text{"\|"} \parallel \text{ExternalID} \parallel \text{"\|"} \parallel \text{SequenceIndex})$$

Where:
- $\text{TenantID}$: Canonical 36-character hyphenated UUID string.
- $\text{AccountID}$: Canonical 36-character hyphenated UUID string.
- $\text{Date}$: ISO 8601 formatted date string: `YYYY-MM-DD`, strictly anchored to Brazilian Official Time (`America/Sao_Paulo`). This ensures late-night transactions (e.g. 22:30 BRT / 01:30 UTC) do not generate divergent fingerprints between cloud workers and local uploads.
- $\text{AmountCents}$: Fixed signed integer string of the amount in cents (e.g. `-154.30` $\to$ `"-15430"`, `+45.00` $\to$ `"4500"`), eliminating decimal formatting divergences.
- $\text{CleanDescription}$: Normalized merchant string with noise tokens, prefixes, and dates removed (uppercase, trimmed).
- $\text{ExternalID}$: FITID or bank reference ID. When unavailable or absent, empty string `""` is used.
- $\text{SequenceIndex}$: Monotonic integer index (stringified, e.g. `'0'`, `'1'`, `'2'`) representing occurrence position or statement row ordinal within the imported batch when `ExternalID` is empty. This prevents legitimate repeat same-day transactions (such as two R$ 20.00 Uber trips or repeated subway fares from CSV) from colliding.

Pipe delimiters (`"|"`) are strictly enforced between all fields to prevent delimiter-free cross-field boundary shifting attacks (e.g. Amount vs Description shifting).

### 4.2 Merchant Description Normalization Algorithm (`CleanDescription`)

```go
package parsers

import (
	"regexp"
	"strings"
)

var (
	stripPrefixes = regexp.MustCompile(`(?i)^(compra\s+cartao|compra\s+com\s+cartao|pix\s+enviado|pix\s+recebido|ted\s+enviada|ted\s+recebida|pagamento\s+boleto|debito\s+automatico|transf\s+eletr\s+ted|doc\s+elet\s+tarifa)\s*[-:]?\s*`)
	stripDates    = regexp.MustCompile(`\b\d{2}/\d{2}(/\d{2,4})?\b`)
	stripSpaces   = regexp.MustCompile(`\s+`)
	stripNoise    = regexp.MustCompile(`[^A-Z0-9\s]`)
)

// CleanDescription strips banking noise, dates, card prefixes, and returns a sanitized uppercase string
func CleanDescription(raw string) string {
	str := strings.ToUpper(raw)
	str = stripPrefixes.ReplaceAllString(str, "")
	str = stripDates.ReplaceAllString(str, "")
	str = stripNoise.ReplaceAllString(str, " ")
	str = stripSpaces.ReplaceAllString(str, " ")
	return strings.TrimSpace(str)
}
```

### 4.3 Database Idempotency & High-Throughput Staging Table Batch Ingestion

In PostgreSQL, the high-speed `COPY` protocol implemented by `pgx.CopyFrom` does **not** support `ON CONFLICT` clauses. Calling `pgx.CopyFrom` directly against the primary `transactions` table with overlapping rows causes PostgreSQL to abort the transaction with error `23505 (unique_violation)`.

To achieve both **ultra-high ingestion throughput** and **atomic deduplication/lifecycle updates**, IntelliFinance implements the **Session-Scoped Unlogged Staging Table Pattern**:

1. A transient unlogged staging table `staging_transactions` is created for the active database transaction (`ON COMMIT DROP`).
2. The batch of parsed and fingerprinted transactions is streamed into `staging_transactions` using `pgx.CopyFrom` (blazing fast, bypassing WAL).
3. An atomic set-based `INSERT INTO transactions ... SELECT ... FROM staging_transactions ON CONFLICT` statement is executed, supporting both duplicate rejection and pending-to-posted transitions.

```go
package ingestion

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type BatchIngestResult struct {
	TotalProcessed int64
	InsertedCount  int64
	UpdatedCount   int64
	DuplicateCount int64
}

func (s *IngestionService) BatchInsertTransactions(
	ctx context.Context,
	tx pgx.Tx,
	txs []*domain.Transaction,
) (*BatchIngestResult, error) {
	if len(txs) == 0 {
		return &BatchIngestResult{}, nil
	}

	// 1. Create session-scoped temporary staging table
	const createStagingSQL = `
		CREATE TEMPORARY TABLE staging_transactions (
			tenant_id UUID NOT NULL,
			account_id UUID NOT NULL,
			category_id UUID,
			created_by_user_id UUID,
			transacted_at TIMESTAMPTZ NOT NULL,
			date DATE NOT NULL,
			amount NUMERIC(15, 2) NOT NULL,
			description VARCHAR(255) NOT NULL,
			clean_description VARCHAR(255) NOT NULL,
			type transaction_type NOT NULL,
			status transaction_status NOT NULL,
			visibility visibility_type NOT NULL,
			source source_type NOT NULL,
			external_id VARCHAR(120),
			sequence_index INT NOT NULL,
			fingerprint VARCHAR(64) NOT NULL,
			metadata JSONB NOT NULL
		) ON COMMIT DROP;
	`
	if _, err := tx.Exec(ctx, createStagingSQL); err != nil {
		return nil, fmt.Errorf("failed to create staging table: %w", err)
	}

	// 2. High-speed bulk load into staging via pgx.CopyFrom
	rows := make([][]any, len(txs))
	for i, t := range txs {
		rows[i] = []any{
			t.TenantID, t.AccountID, t.CategoryID, t.CreatedByUserID,
			t.TransactedAt, t.Date, t.Amount, t.Description, t.CleanDescription, t.Type,
			t.Status, t.Visibility, t.Source, t.ExternalID, t.SequenceIndex, t.Fingerprint, t.Metadata,
		}
	}

	copyCount, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"staging_transactions"},
		[]string{
			"tenant_id", "account_id", "category_id", "created_by_user_id",
			"transacted_at", "date", "amount", "description", "clean_description", "type",
			"status", "visibility", "source", "external_id", "sequence_index", "fingerprint", "metadata",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to copy rows into staging table: %w", err)
	}

	// 3. Set-based atomic upsert into production ledger with pending-to-posted transition
	const upsertSQL = `
		WITH inserted AS (
			INSERT INTO transactions (
				tenant_id, account_id, category_id, created_by_user_id,
				transacted_at, date, amount, description, clean_description, type,
				status, visibility, source, external_id, sequence_index, fingerprint, metadata
			)
			SELECT 
				tenant_id, account_id, category_id, created_by_user_id,
				transacted_at, date, amount, description, clean_description, type,
				status, visibility, source, external_id, sequence_index, fingerprint, metadata
			FROM staging_transactions
			ON CONFLICT (tenant_id, fingerprint) DO UPDATE
			SET 
				status = EXCLUDED.status,
				updated_at = NOW()
			WHERE transactions.status = 'PENDING' AND EXCLUDED.status != 'PENDING'
			RETURNING id, (xmax = 0) AS was_inserted
		)
		SELECT 
			COUNT(*) FILTER (WHERE was_inserted = TRUE) AS inserted_count,
			COUNT(*) FILTER (WHERE was_inserted = FALSE) AS updated_count
		FROM inserted;
	`
	var insertedCount, updatedCount int64
	if err := tx.QueryRow(ctx, upsertSQL).Scan(&insertedCount, &updatedCount); err != nil {
		return nil, fmt.Errorf("failed executing staging-to-ledger upsert: %w", err)
	}

	duplicateCount := copyCount - insertedCount - updatedCount

	return &BatchIngestResult{
		TotalProcessed: copyCount,
		InsertedCount:  insertedCount,
		UpdatedCount:   updatedCount,
		DuplicateCount: duplicateCount,
	}, nil
}
```
