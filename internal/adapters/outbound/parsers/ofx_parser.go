package parsers

import (
	"bytes"
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
	reOFXBankID   = regexp.MustCompile(`(?i)<BANKID>([^<\r\n]+)`)
	reOFXAcctID   = regexp.MustCompile(`(?i)<ACCTID>([^<\r\n]+)`)
	reOFXCurDef   = regexp.MustCompile(`(?i)<CURDEF>([^<\r\n]+)`)
	reOFXDtStart  = regexp.MustCompile(`(?i)<DTSTART>([^<\r\n]+)`)
	reOFXDtEnd    = regexp.MustCompile(`(?i)<DTEND>([^<\r\n]+)`)
	reOFXBalAmt   = regexp.MustCompile(`(?i)<BALAMT>([^<\r\n]+)`)
	reOFXStmtTrn  = regexp.MustCompile(`(?i)<STMTTRN>`)
	reOFXEndBlock = regexp.MustCompile(`(?i)(</STMTTRN>|<STMTTRN>|</BANKTRANLIST>|<LEDGERBAL>|</STMTRS>|</CCSTMTRS>)`)
	reOFXTrnAmt   = regexp.MustCompile(`(?i)<TRNAMT>([^<\r\n]+)`)
	reOFXDtPosted = regexp.MustCompile(`(?i)<DTPOSTED>([^<\r\n]+)`)
	reOFXFitID    = regexp.MustCompile(`(?i)<FITID>([^<\r\n]+)`)
	reOFXMemo     = regexp.MustCompile(`(?i)<MEMO>([^<\r\n]+)`)
	reOFXName     = regexp.MustCompile(`(?i)<NAME>([^<\r\n]+)`)
	reOFXTrnType  = regexp.MustCompile(`(?i)<TRNTYPE>([^<\r\n]+)`)
	reOFXCheckNum = regexp.MustCompile(`(?i)<CHECKNUM>([^<\r\n]+)`)
	reOFXRefNum   = regexp.MustCompile(`(?i)<REFNUM>([^<\r\n]+)`)
)

// OFXParser implements ports.StatementParser for OFX 1.02 (SGML) and 2.x (XML) statements.
type OFXParser struct{}

// NewOFXParser creates a new stateless OFXParser instance.
func NewOFXParser() *OFXParser {
	return &OFXParser{}
}

// Format returns ports.SourceFormatOFX.
func (p *OFXParser) Format() ports.SourceFormat {
	return ports.SourceFormatOFX
}

// CanParse sniffs whether the file or stream is an OFX statement.
func (p *OFXParser) CanParse(filename string, r io.Reader) (bool, error) {
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".ofx") {
		return true, nil
	}
	if r == nil {
		return false, nil
	}

	buf := make([]byte, 512)
	n, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read ofx sniff buffer: %w", err)
	}

	sample := strings.ToUpper(string(buf[:n]))
	return strings.Contains(sample, "OFXHEADER") || strings.Contains(sample, "<OFX>"), nil
}

// Parse reads and normalizes an OFX 1.02 or 2.x statement stream.
func (p *OFXParser) Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ports.ParsedStatement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()

	if r == nil {
		return nil, fmt.Errorf("ofx reader is nil")
	}

	rawBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read ofx stream: %w", err)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	if len(bytes.TrimSpace(rawBytes)) == 0 {
		return nil, fmt.Errorf("empty ofx stream")
	}

	decodedText, err := p.decodeToUTF8(rawBytes)
	if err != nil {
		return nil, fmt.Errorf("ofx charset decoding failure: %w", err)
	}

	upperText := strings.ToUpper(decodedText)
	if !strings.Contains(upperText, "<OFX>") && !strings.Contains(upperText, "<STMTTRN>") && !strings.Contains(upperText, "OFXHEADER") {
		return nil, fmt.Errorf("invalid ofx document: missing <OFX> root or <STMTTRN> records")
	}

	stmt := &ports.ParsedStatement{
		Format:       ports.SourceFormatOFX,
		Currency:     "BRL",
		Transactions: make([]ports.RawTransaction, 0, 100),
	}

	if defaultAccountID != uuid.Nil {
		stmt.AccountID = defaultAccountID.String()
	}

	p.extractHeaderMetadata(decodedText, stmt)

	blocks := p.extractTransactionBlocks(decodedText)
	for idx, block := range blocks {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		rawTx, parseErr := p.parseTransactionBlock(block)
		if parseErr != nil {
			stmt.SkippedRows++
			stmt.ValidationWarns = append(stmt.ValidationWarns, fmt.Sprintf("stmttrn[%d]: %v", idx+1, parseErr))
			continue
		}
		stmt.Transactions = append(stmt.Transactions, rawTx)
	}

	return stmt, nil
}

func (p *OFXParser) decodeToUTF8(raw []byte) (string, error) {
	headerLen := len(raw)
	if headerLen > 1024 {
		headerLen = 1024
	}
	upperHeader := bytes.ToUpper(raw[:headerLen])

	declaresLatin1 := bytes.Contains(upperHeader, []byte("CHARSET:1252")) ||
		bytes.Contains(upperHeader, []byte("CHARSET:ISO-8859-1")) ||
		bytes.Contains(upperHeader, []byte("CHARSET:8859-1")) ||
		bytes.Contains(upperHeader, []byte("ENCODING:USASCII")) ||
		bytes.Contains(upperHeader, []byte("ENCODING:IBMPC"))

	if !utf8.Valid(raw) || (declaresLatin1 && !utf8.Valid(raw)) {
		decoded, err := charmap.Windows1252.NewDecoder().Bytes(raw)
		if err != nil {
			return "", fmt.Errorf("windows-1252 decode error: %w", err)
		}
		return string(decoded), nil
	}

	return string(raw), nil
}

func (p *OFXParser) extractHeaderMetadata(text string, stmt *ports.ParsedStatement) {
	if val := extractRegexTag(reOFXBankID, text); val != "" {
		stmt.BankID = val
	}
	if val := extractRegexTag(reOFXAcctID, text); val != "" {
		stmt.AccountID = val
	}
	if val := extractRegexTag(reOFXCurDef, text); val != "" {
		stmt.Currency = val
	}
	if val := extractRegexTag(reOFXDtStart, text); val != "" {
		if dt, err := parseFlexibleDate(val); err == nil {
			stmt.PeriodStart = dt
		}
	}
	if val := extractRegexTag(reOFXDtEnd, text); val != "" {
		if dt, err := parseFlexibleDate(val); err == nil {
			stmt.PeriodEnd = dt
		}
	}
	if val := extractRegexTag(reOFXBalAmt, text); val != "" {
		if bal, err := parseMonetaryAmount(val); err == nil {
			stmt.HeaderBalance = &bal
		}
	}
}

func (p *OFXParser) extractTransactionBlocks(text string) []string {
	locs := reOFXStmtTrn.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return nil
	}

	blocks := make([]string, 0, len(locs))
	for _, loc := range locs {
		contentStart := loc[1]
		remaining := text[contentStart:]
		endLoc := reOFXEndBlock.FindStringIndex(remaining)
		var block string
		if endLoc != nil {
			block = remaining[:endLoc[0]]
		} else {
			block = remaining
		}
		if strings.TrimSpace(block) != "" {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func (p *OFXParser) parseTransactionBlock(block string) (ports.RawTransaction, error) {
	rawAmt := extractRegexTag(reOFXTrnAmt, block)
	if rawAmt == "" {
		return ports.RawTransaction{}, fmt.Errorf("missing TRNAMT tag")
	}
	amount, err := parseMonetaryAmount(rawAmt)
	if err != nil {
		return ports.RawTransaction{}, fmt.Errorf("invalid TRNAMT %q: %w", rawAmt, err)
	}

	rawDate := extractRegexTag(reOFXDtPosted, block)
	if rawDate == "" {
		return ports.RawTransaction{}, fmt.Errorf("missing DTPOSTED tag")
	}
	parsedDate, err := parseFlexibleDate(rawDate)
	if err != nil {
		return ports.RawTransaction{}, fmt.Errorf("invalid DTPOSTED %q: %w", rawDate, err)
	}

	memo := extractRegexTag(reOFXMemo, block)
	if memo == "" {
		memo = extractRegexTag(reOFXName, block)
	}

	var meta map[string]any
	checkNum := extractRegexTag(reOFXCheckNum, block)
	refNum := extractRegexTag(reOFXRefNum, block)
	trnType := extractRegexTag(reOFXTrnType, block)
	if checkNum != "" || refNum != "" || trnType != "" {
		meta = make(map[string]any)
		if trnType != "" {
			meta["trn_type"] = trnType
		}
		if checkNum != "" {
			meta["check_num"] = checkNum
		}
		if refNum != "" {
			meta["ref_num"] = refNum
		}
	}

	return ports.RawTransaction{
		ExternalID:       extractRegexTag(reOFXFitID, block),
		Date:             parsedDate,
		Amount:           amount,
		Description:      memo,
		CleanDescription: cleanMerchantName(memo),
		Metadata:         meta,
	}, nil
}

func extractRegexTag(re *regexp.Regexp, input string) string {
	m := re.FindStringSubmatch(input)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
