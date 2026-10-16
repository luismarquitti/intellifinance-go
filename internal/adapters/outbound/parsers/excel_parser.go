package parsers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var xlsxMagicHeader = []byte{0x50, 0x4B, 0x03, 0x04}

// ExcelParser implements ports.StatementParser for XLSX workbooks using streaming row iterators.
type ExcelParser struct {
	csvParser *CSVParser
}

// NewExcelParser creates a new ExcelParser instance.
func NewExcelParser() *ExcelParser {
	return &ExcelParser{
		csvParser: NewCSVParser(),
	}
}

// Format returns ports.SourceFormatExcel.
func (p *ExcelParser) Format() ports.SourceFormat {
	return ports.SourceFormatExcel
}

// CanParse sniffs whether the file or stream is an XLSX workbook.
func (p *ExcelParser) CanParse(filename string, r io.Reader) (bool, error) {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".xlsx") {
		return true, nil
	}
	if r == nil {
		return false, nil
	}

	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read xlsx sniff buffer: %w", err)
	}

	return n >= 4 && bytes.HasPrefix(buf[:n], xlsxMagicHeader), nil
}

// Parse streams and normalizes transactions from an XLSX workbook using excelize.Rows().
func (p *ExcelParser) Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (stmt *ports.ParsedStatement, err error) {
	if err = ctx.Err(); err != nil {
		return nil, err
	}

	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()

	if r == nil {
		return nil, fmt.Errorf("excel reader is nil")
	}

	f, openErr := excelize.OpenReader(r)
	if openErr != nil {
		return nil, fmt.Errorf("failed to open excel workbook: %w", openErr)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close excel workbook: %w", closeErr)
		}
	}()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("excel workbook contains no sheets")
	}

	stmt = &ports.ParsedStatement{
		Format:       ports.SourceFormatExcel,
		Currency:     "BRL",
		Transactions: make([]ports.RawTransaction, 0, 100),
	}
	if defaultAccountID != uuid.Nil {
		stmt.AccountID = defaultAccountID.String()
	}

	headerIdentified := false
	for _, sheetName := range sheets {
		foundInSheet, sheetErr := p.parseSheet(ctx, f, sheetName, stmt)
		if sheetErr != nil {
			return nil, sheetErr
		}
		if foundInSheet {
			headerIdentified = true
			break
		}
	}

	if !headerIdentified {
		return nil, fmt.Errorf("unable to map mandatory excel columns (date, amount, description)")
	}

	return stmt, nil
}

func (p *ExcelParser) parseSheet(
	ctx context.Context,
	f *excelize.File,
	sheetName string,
	stmt *ports.ParsedStatement,
) (headerFound bool, err error) {
	rows, rowsErr := f.Rows(sheetName)
	if rowsErr != nil {
		return false, fmt.Errorf("failed to stream rows in sheet %q: %w", sheetName, rowsErr)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close row iterator for sheet %q: %w", sheetName, closeErr)
		}
	}()

	var colMap columnMapping
	var isNubankCredit bool
	rowIndex := 0

	for rows.Next() {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return headerFound, ctxErr
		}

		rowIndex++
		row, colErr := rows.Columns()
		if colErr != nil {
			return headerFound, fmt.Errorf("failed to read columns on row %d of sheet %q: %w", rowIndex, sheetName, colErr)
		}

		if isRowEmpty(row) {
			continue
		}

		if !headerFound {
			candidateMap, nubank := p.csvParser.identifyColumns(row)
			if candidateMap.isValid() {
				colMap = candidateMap
				isNubankCredit = nubank
				headerFound = true
			}
			continue
		}

		maxReqIdx := colMap.maxRequiredIndex()
		if len(row) <= maxReqIdx {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("sheet %q row %d: insufficient columns", sheetName, rowIndex))
			continue
		}

		parsedDate, dateErr := parseFlexibleDate(row[colMap.dateIdx])
		if dateErr != nil {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("sheet %q row %d: invalid date %q: %v", sheetName, rowIndex, row[colMap.dateIdx], dateErr))
			continue
		}

		amount, amtErr := parseMonetaryAmount(row[colMap.amountIdx])
		if amtErr != nil {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("sheet %q row %d: invalid amount %q: %v", sheetName, rowIndex, row[colMap.amountIdx], amtErr))
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

	if iterErr := rows.Error(); iterErr != nil {
		return headerFound, fmt.Errorf("row iteration error in sheet %q: %w", sheetName, iterErr)
	}

	return headerFound, nil
}
