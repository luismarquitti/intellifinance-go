package parsers

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/encoding/charmap"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var (
	pdfMagicSignature = []byte("%PDF-")
	rePDFTxLine       = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4}|\d{4}-\d{2}-\d{2}|\d{2}-\d{2}-\d{4}|\d{2}/\d{2}/\d{2}|\d{2}/\d{2})\s+(.+?)\s+(?:R\$\s*)?(\(?[-+]?\s*(?:R\$\s*)?\d[\d.,]*\)?(?:\s*[DCdc])?)$`)
	rePDFTextOp       = regexp.MustCompile(`(?s)\[(.*?)\]\s*TJ|\((?:\\.|[^\\()])*\)\s*(?:Tj|'|")`)
	rePDFStringLit    = regexp.MustCompile(`\((?:\\.|[^\\()])*\)`)
)

// PDFParser implements ports.StatementParser for digital PDF statements and credit card invoices.
type PDFParser struct{}

// NewPDFParser creates a new stateless PDFParser instance.
func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

// Format returns ports.SourceFormatPDF.
func (p *PDFParser) Format() ports.SourceFormat {
	return ports.SourceFormatPDF
}

// CanParse sniffs whether the file or stream is a PDF document.
func (p *PDFParser) CanParse(filename string, r io.Reader) (bool, error) {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".pdf") {
		return true, nil
	}
	if r == nil {
		return false, nil
	}

	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read pdf sniff buffer: %w", err)
	}

	return bytes.Contains(buf[:n], pdfMagicSignature), nil
}

// Parse extracts and normalizes transaction lines from digital PDF streams.
func (p *PDFParser) Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ports.ParsedStatement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()

	if r == nil {
		return nil, fmt.Errorf("pdf reader is nil")
	}

	rawBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read pdf stream: %w", err)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	headerCheckLen := len(rawBytes)
	if headerCheckLen > 1024 {
		headerCheckLen = 1024
	}
	if headerCheckLen == 0 || !bytes.Contains(rawBytes[:headerCheckLen], pdfMagicSignature) {
		return nil, fmt.Errorf("invalid PDF header: missing %%PDF- signature")
	}

	lines, err := p.extractTextLines(ctx, rawBytes)
	if err != nil {
		return nil, err
	}

	stmt := &ports.ParsedStatement{
		Format:       ports.SourceFormatPDF,
		Currency:     "BRL",
		Transactions: make([]ports.RawTransaction, 0, len(lines)),
	}
	if defaultAccountID != uuid.Nil {
		stmt.AccountID = defaultAccountID.String()
	}

	for _, line := range lines {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		tx, matched, parseErr := p.parseTransactionLine(trimmed)
		if !matched {
			continue
		}
		if parseErr != nil {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, parseErr.Error())
			continue
		}

		stmt.Transactions = append(stmt.Transactions, tx)
	}

	return stmt, nil
}

func (p *PDFParser) extractTextLines(ctx context.Context, rawPDF []byte) ([]string, error) {
	streams := extractPDFStreams(rawPDF)
	if len(streams) == 0 {
		streams = [][]byte{rawPDF}
	}

	var allLines []string
	for _, streamBytes := range streams {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		decodedStream := tryDecompressStream(streamBytes)
		streamText := ensureUTF8String(decodedStream)
		linesFromOps := extractLinesFromPDFOperators(streamText)
		if len(linesFromOps) > 0 {
			allLines = append(allLines, linesFromOps...)
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(streamText))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				allLines = append(allLines, line)
			}
		}
	}

	return allLines, nil
}

func extractPDFStreams(rawPDF []byte) [][]byte {
	var streams [][]byte
	searchData := rawPDF

	for {
		streamIdx := bytes.Index(searchData, []byte("stream"))
		if streamIdx == -1 {
			break
		}

		// Ensure "stream" is not part of "endstream"
		if streamIdx >= 3 && bytes.Equal(searchData[streamIdx-3:streamIdx], []byte("end")) {
			searchData = searchData[streamIdx+6:]
			continue
		}

		contentStart := streamIdx + 6
		if contentStart < len(searchData) && searchData[contentStart] == '\r' {
			contentStart++
		}
		if contentStart < len(searchData) && searchData[contentStart] == '\n' {
			contentStart++
		}

		remaining := searchData[contentStart:]
		endIdx := bytes.Index(remaining, []byte("endstream"))
		if endIdx == -1 {
			break
		}

		streamContent := remaining[:endIdx]
		streamContent = bytes.TrimSuffix(streamContent, []byte("\n"))
		streamContent = bytes.TrimSuffix(streamContent, []byte("\r"))
		streams = append(streams, streamContent)

		searchData = remaining[endIdx+9:]
	}

	return streams
}

func tryDecompressStream(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}

	if zr, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
		decompressed, readErr := io.ReadAll(zr)
		closeErr := zr.Close()
		if readErr == nil && closeErr == nil && len(decompressed) > 0 {
			return decompressed
		}
	}

	fr := flate.NewReader(bytes.NewReader(raw))
	decompressed, readErr := io.ReadAll(fr)
	closeErr := fr.Close()
	if readErr == nil && closeErr == nil && len(decompressed) > 0 {
		return decompressed
	}

	return raw
}

func extractLinesFromPDFOperators(content string) []string {
	var extracted []string
	var pendingFragments []string

	flushFragments := func() {
		if len(pendingFragments) > 0 {
			combined := strings.TrimSpace(strings.Join(pendingFragments, " "))
			if combined != "" {
				extracted = append(extracted, combined)
			}
			pendingFragments = nil
		}
	}

	opMatches := rePDFTextOp.FindAllStringSubmatch(content, -1)
	for _, m := range opMatches {
		fullMatch := m[0]
		if strings.HasPrefix(fullMatch, "[") {
			flushFragments()
			lits := rePDFStringLit.FindAllString(m[1], -1)
			var sb strings.Builder
			for _, lit := range lits {
				sb.WriteString(unescapePDFString(lit))
			}
			joined := strings.TrimSpace(sb.String())
			if joined != "" {
				for _, subLine := range strings.Split(joined, "\n") {
					if trimmed := strings.TrimSpace(subLine); trimmed != "" {
						extracted = append(extracted, trimmed)
					}
				}
			}
			continue
		}

		lit := rePDFStringLit.FindString(fullMatch)
		if lit == "" {
			continue
		}
		decoded := unescapePDFString(lit)
		for _, subLine := range strings.Split(decoded, "\n") {
			trimmed := strings.TrimSpace(subLine)
			if trimmed == "" {
				continue
			}
			if rePDFTxLine.MatchString(trimmed) {
				flushFragments()
				extracted = append(extracted, trimmed)
			} else {
				pendingFragments = append(pendingFragments, trimmed)
				candidate := strings.Join(pendingFragments, " ")
				if rePDFTxLine.MatchString(candidate) {
					extracted = append(extracted, candidate)
					pendingFragments = nil
				}
			}
		}
	}
	flushFragments()

	return extracted
}

func unescapePDFString(lit string) string {
	if len(lit) >= 2 && lit[0] == '(' && lit[len(lit)-1] == ')' {
		lit = lit[1 : len(lit)-1]
	}

	var out bytes.Buffer
	for i := 0; i < len(lit); i++ {
		if lit[i] != '\\' || i+1 >= len(lit) {
			out.WriteByte(lit[i])
			continue
		}
		i++
		switch lit[i] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case '(', ')', '\\':
			out.WriteByte(lit[i])
		default:
			out.WriteByte(lit[i])
		}
	}

	return ensureUTF8String(out.Bytes())
}

func ensureUTF8String(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(decoded)
}

func (p *PDFParser) parseTransactionLine(line string) (ports.RawTransaction, bool, error) {
	m := rePDFTxLine.FindStringSubmatch(line)
	if len(m) != 4 {
		return ports.RawTransaction{}, false, nil
	}

	rawDate := strings.TrimSpace(m[1])
	rawDesc := strings.TrimSpace(m[2])
	rawAmt := strings.TrimSpace(m[3])

	parsedDate, err := parseFlexibleDate(rawDate)
	if err != nil {
		return ports.RawTransaction{}, true, fmt.Errorf("invalid pdf transaction date %q: %w", rawDate, err)
	}

	amount, err := parseMonetaryAmount(rawAmt)
	if err != nil {
		return ports.RawTransaction{}, true, fmt.Errorf("invalid pdf transaction amount %q: %w", rawAmt, err)
	}

	return ports.RawTransaction{
		Date:             parsedDate,
		Amount:           amount,
		Description:      rawDesc,
		CleanDescription: cleanMerchantName(rawDesc),
	}, true, nil
}
