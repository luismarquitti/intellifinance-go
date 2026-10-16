package parsers

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

const utf8BOM = "\xef\xbb\xbf"

type columnMapping struct {
	dateIdx     int
	amountIdx   int
	descIdx     int
	idIdx       int
	categoryIdx int
	accountIdx  int
}

func (m columnMapping) isValid() bool {
	return m.dateIdx != -1 && m.amountIdx != -1 && m.descIdx != -1
}

func (m columnMapping) maxRequiredIndex() int {
	maxIdx := m.dateIdx
	if m.amountIdx > maxIdx {
		maxIdx = m.amountIdx
	}
	if m.descIdx > maxIdx {
		maxIdx = m.descIdx
	}
	return maxIdx
}

// CSVParser implements ports.StatementParser for Brazilian and international CSV statements.
type CSVParser struct{}

// NewCSVParser creates a new stateless CSVParser instance.
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// Format returns ports.SourceFormatCSV.
func (p *CSVParser) Format() ports.SourceFormat {
	return ports.SourceFormatCSV
}

// CanParse sniffs whether the file or stream is a supported CSV statement.
func (p *CSVParser) CanParse(filename string, r io.Reader) (bool, error) {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".csv") {
		return true, nil
	}
	if r == nil {
		return false, nil
	}

	buf := make([]byte, 2048)
	n, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read csv sniff buffer: %w", err)
	}

	sample := strings.TrimPrefix(string(buf[:n]), utf8BOM)
	delim := p.detectDelimiter(sample)

	scanner := bufio.NewScanner(strings.NewReader(sample))
	checkedLines := 0
	for scanner.Scan() && checkedLines < 15 {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		checkedLines++
		fields := strings.Split(line, string(delim))
		colMap, _ := p.identifyColumns(fields)
		if colMap.isValid() {
			return true, nil
		}
	}

	return false, nil
}

// Parse reads and normalizes a CSV statement stream with auto-delimiter detection and Brazilian decimal support.
func (p *CSVParser) Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ports.ParsedStatement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()

	if r == nil {
		return nil, fmt.Errorf("csv reader is nil")
	}

	br := bufio.NewReader(r)
	sampleBytes, err := br.Peek(2048)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, fmt.Errorf("failed to peek csv stream: %w", err)
	}
	if len(sampleBytes) == 0 {
		return nil, fmt.Errorf("empty csv stream")
	}

	delimiter := p.detectDelimiter(string(sampleBytes))

	csvReader := csv.NewReader(br)
	csvReader.Comma = delimiter
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	var colMap columnMapping
	var isNubankCredit bool
	headerFound := false
	lineNum := 0
	nonEmptyChecked := 0

	for nonEmptyChecked < 15 {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		row, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("csv header read error on line %d: %w", lineNum+1, readErr)
		}
		lineNum++

		if isRowEmpty(row) {
			continue
		}
		nonEmptyChecked++

		candidateMap, nubank := p.identifyColumns(row)
		if candidateMap.isValid() {
			colMap = candidateMap
			isNubankCredit = nubank
			headerFound = true
			break
		}
	}

	if !headerFound {
		return nil, fmt.Errorf("unable to map mandatory csv columns (date, amount, description)")
	}

	stmt := &ports.ParsedStatement{
		Format:       ports.SourceFormatCSV,
		Currency:     "BRL",
		Transactions: make([]ports.RawTransaction, 0, 100),
	}
	if defaultAccountID != uuid.Nil {
		stmt.AccountID = defaultAccountID.String()
	}

	maxRequiredIdx := colMap.maxRequiredIndex()

	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		row, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		lineNum++
		if readErr != nil {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("line %d: csv read error: %v", lineNum, readErr))
			continue
		}

		if isRowEmpty(row) {
			continue
		}

		if len(row) <= maxRequiredIdx {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("line %d: insufficient columns (got %d, need at least %d)", lineNum, len(row), maxRequiredIdx+1))
			continue
		}

		parsedDate, dateErr := parseFlexibleDate(row[colMap.dateIdx])
		if dateErr != nil {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("line %d: invalid date %q: %v", lineNum, row[colMap.dateIdx], dateErr))
			continue
		}

		amount, amtErr := parseMonetaryAmount(row[colMap.amountIdx])
		if amtErr != nil {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("line %d: invalid amount %q: %v", lineNum, row[colMap.amountIdx], amtErr))
			continue
		}

		if isNubankCredit {
			amount = amount.Neg()
		}

		desc := strings.TrimSpace(row[colMap.descIdx])

		var extID, catHint, acctHint string
		if colMap.idIdx != -1 && colMap.idIdx < len(row) {
			extID = strings.TrimSpace(row[colMap.idIdx])
		}
		if colMap.categoryIdx != -1 && colMap.categoryIdx < len(row) {
			catHint = strings.TrimSpace(row[colMap.categoryIdx])
		}
		if colMap.accountIdx != -1 && colMap.accountIdx < len(row) {
			acctHint = strings.TrimSpace(row[colMap.accountIdx])
		}

		stmt.Transactions = append(stmt.Transactions, ports.RawTransaction{
			ExternalID:       extID,
			Date:             parsedDate,
			Amount:           amount,
			Description:      desc,
			CleanDescription: cleanMerchantName(desc),
			CategoryHint:     catHint,
			AccountHint:      acctHint,
		})
	}

	return stmt, nil
}

func (p *CSVParser) detectDelimiter(sample string) rune {
	cleanSample := strings.TrimPrefix(sample, utf8BOM)
	scanner := bufio.NewScanner(strings.NewReader(cleanSample))

	var firstNonEmpty string
	checked := 0
	for scanner.Scan() && checked < 15 {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		checked++
		if firstNonEmpty == "" {
			firstNonEmpty = line
		}

		for _, candidate := range []rune{';', ',', '\t'} {
			fields := strings.Split(line, string(candidate))
			if m, _ := p.identifyColumns(fields); m.isValid() {
				return candidate
			}
		}
	}

	target := firstNonEmpty
	if target == "" {
		target = cleanSample
	}

	commaCount := strings.Count(target, ",")
	semicolonCount := strings.Count(target, ";")
	tabCount := strings.Count(target, "\t")

	if semicolonCount > commaCount && semicolonCount >= tabCount && semicolonCount > 0 {
		return ';'
	}
	if tabCount > commaCount && tabCount > semicolonCount {
		return '\t'
	}
	return ','
}

//nolint:misspell // Portuguese header column names ("historico", "lancamento", "transacao")
func (p *CSVParser) identifyColumns(header []string) (columnMapping, bool) {
	mapping := columnMapping{
		dateIdx:     -1,
		amountIdx:   -1,
		descIdx:     -1,
		idIdx:       -1,
		categoryIdx: -1,
		accountIdx:  -1,
	}

	normalized := make([]string, len(header))
	for i, col := range header {
		clean := strings.TrimPrefix(col, utf8BOM)
		clean = strings.ToLower(strings.TrimSpace(clean))
		clean = strings.Trim(clean, `"'`)
		normalized[i] = clean

		switch clean {
		case "data", "date", "data lançamento", "data lancamento", "dt. lancamento", "dt lancamento", "data da transação", "data transacao":
			if mapping.dateIdx == -1 {
				mapping.dateIdx = i
			}
		case "valor", "amount", "valor (r$)", "valor r$", "value":
			if mapping.amountIdx == -1 {
				mapping.amountIdx = i
			}
		case "descrição", "descricao", "description", "histórico", "historico", "detalhes", "lançamento", "lancamento", "title", "estabelecimento", "memo":
			if mapping.descIdx == -1 {
				mapping.descIdx = i
			}
		case "identificador", "id", "documento", "doc", "fitid", "external_id":
			if mapping.idIdx == -1 {
				mapping.idIdx = i
			}
		case "categoria", "category":
			if mapping.categoryIdx == -1 {
				mapping.categoryIdx = i
			}
		case "conta", "account":
			if mapping.accountIdx == -1 {
				mapping.accountIdx = i
			}
		}
	}

	isNubankCredit := len(normalized) == 4 &&
		normalized[0] == "date" &&
		normalized[1] == "category" &&
		normalized[2] == "title" &&
		normalized[3] == "amount"

	return mapping, isNubankCredit
}

func isRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(strings.TrimPrefix(cell, utf8BOM)) != "" {
			return false
		}
	}
	return true
}
