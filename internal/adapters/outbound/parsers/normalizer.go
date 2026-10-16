package parsers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/shopspring/decimal"

	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// Compile-time interface assertions.
var (
	_ ports.StatementParser = (*OFXParser)(nil)
	_ ports.StatementParser = (*CSVParser)(nil)
	_ ports.StatementParser = (*ExcelParser)(nil)
	_ ports.StatementParser = (*PDFParser)(nil)
)

// DefaultParseTimeout defines the fallback execution deadline applied when a caller context has no deadline.
const DefaultParseTimeout = 30 * time.Second

//nolint:misspell // Portuguese banking keywords in regex pattern
var (
	reStripPrefixes = regexp.MustCompile(`(?i)^(compra\s+com\s+cart[aã]o|compra\s+cart[aã]o|pagto\s+eletro|pix\s+transf|pix\s+enviado|pix\s+recebido|ted\s+enviada|ted\s+recebida|transf\s+eletr\s+ted|doc\s+elet\s+tarifa|pagamento\s+de\s+boleto|pagamento\s+boleto|d[eé]bito\s+autom[aá]tico|deb\s+autom[aá]tico|ted\b|doc\b)\s*[-:]?\s*`)
	reStripDates    = regexp.MustCompile(`\b\d{2}/\d{2}(/\d{2,4})?\b`)
	reStripNoise    = regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	reStripSpaces   = regexp.MustCompile(`\s+`)
)

var supportedDateLayouts = []string{
	"02/01/2006",
	"2006-01-02",
	"02-01-2006",
	"2006/01/02",
	"02.01.2006",
	"02/01/06",
	"01-02-06",
	"20060102",
	time.RFC3339,
}

// ensureContextTimeout ensures the provided context has a bounded execution deadline.
func ensureContextTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		return context.WithTimeout(ctx, DefaultParseTimeout)
	}
	return context.WithCancel(ctx)
}

// CleanMerchantName normalizes raw bank transaction descriptions into canonical merchant names.
func CleanMerchantName(raw string) string {
	str := strings.ToUpper(strings.TrimSpace(raw))
	for {
		next := reStripPrefixes.ReplaceAllString(str, "")
		if next == str {
			break
		}
		str = strings.TrimSpace(next)
	}
	str = reStripDates.ReplaceAllString(str, " ")
	str = reStripNoise.ReplaceAllString(str, " ")
	str = reStripSpaces.ReplaceAllString(str, " ")
	return strings.TrimSpace(str)
}

// cleanMerchantName is the package-internal alias required by the task specification.
func cleanMerchantName(raw string) string {
	return CleanMerchantName(raw)
}

// CleanDescription delegates to CleanMerchantName for compatibility with ingestion specs.
func CleanDescription(raw string) string {
	return CleanMerchantName(raw)
}

// parseMonetaryAmount parses a Brazilian or international currency string into an exact decimal.Decimal.
func parseMonetaryAmount(raw string) (decimal.Decimal, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return decimal.Zero, fmt.Errorf("empty monetary amount")
	}

	isNegative := false

	// Check accounting parentheses before removing currency symbols, e.g. "(R$ 1.234,56)" or "R$ (1.234,56)"
	if strings.HasPrefix(clean, "(") && strings.HasSuffix(clean, ")") {
		isNegative = true
		clean = strings.TrimSpace(clean[1 : len(clean)-1])
	}

	// Check trailing accounting Debit/Credit indicator (D/C) when preceded by space or digit
	if len(clean) >= 2 {
		lastChar := clean[len(clean)-1]
		prevChar := clean[len(clean)-2]
		if (lastChar == 'D' || lastChar == 'd') && (prevChar == ' ' || unicode.IsDigit(rune(prevChar))) {
			isNegative = true
			clean = strings.TrimSpace(clean[:len(clean)-1])
		} else if (lastChar == 'C' || lastChar == 'c') && (prevChar == ' ' || unicode.IsDigit(rune(prevChar))) {
			clean = strings.TrimSpace(clean[:len(clean)-1])
		}
	}

	// Remove currency symbols and non-breaking spaces
	replacer := strings.NewReplacer(
		"R$", "",
		"r$", "",
		"BRL", "",
		"brl", "",
		"$", "",
		"\u00a0", "",
		" ", "",
		"\t", "",
	)
	clean = strings.TrimSpace(replacer.Replace(clean))

	// Check parentheses again in case currency symbol was outside parentheses: R$ (1.234,56)
	if strings.HasPrefix(clean, "(") && strings.HasSuffix(clean, ")") {
		isNegative = true
		clean = strings.TrimSpace(clean[1 : len(clean)-1])
	}

	if strings.HasPrefix(clean, "-") {
		isNegative = true
		clean = strings.TrimPrefix(clean, "-")
	} else if strings.HasSuffix(clean, "-") {
		isNegative = true
		clean = strings.TrimSuffix(clean, "-")
	} else if strings.HasPrefix(clean, "+") {
		clean = strings.TrimPrefix(clean, "+")
	}

	if clean == "" {
		return decimal.Zero, fmt.Errorf("invalid monetary amount %q", raw)
	}

	// Normalize thousand and decimal separators
	hasDot := strings.Contains(clean, ".")
	hasComma := strings.Contains(clean, ",")

	switch {
	case hasDot && hasComma:
		if strings.LastIndex(clean, ",") > strings.LastIndex(clean, ".") {
			// Brazilian format: 1.234.567,89
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.ReplaceAll(clean, ",", ".")
		} else {
			// International format: 1,234,567.89
			clean = strings.ReplaceAll(clean, ",", "")
		}
	case hasComma:
		if strings.Count(clean, ",") == 1 {
			clean = strings.ReplaceAll(clean, ",", ".")
		} else {
			clean = strings.ReplaceAll(clean, ",", "")
		}
	case hasDot:
		if strings.Count(clean, ".") > 1 {
			clean = strings.ReplaceAll(clean, ".", "")
		}
	}

	dec, err := decimal.NewFromString(clean)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid monetary amount %q: %w", raw, err)
	}

	if isNegative && dec.IsPositive() {
		dec = dec.Neg()
	}

	return dec, nil
}

// parseFlexibleDate parses Brazilian, ISO, Excel, and OFX date representations into a normalized UTC date.
func parseFlexibleDate(raw string) (time.Time, error) {
	return parseFlexibleDateWithYear(raw, 2025)
}

// parseFlexibleDateWithYear parses date representations and resolves day/month-only dates using defaultYear.
func parseFlexibleDateWithYear(raw string, defaultYear int) (time.Time, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	// Handle OFX timestamps starting with YYYYMMDD (e.g. 20250421120000[-3:BRT])
	if len(clean) >= 8 && isASCIIAllDigits(clean[:8]) {
		t, err := time.Parse("20060102", clean[:8])
		if err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}

	for _, layout := range supportedDateLayouts {
		if t, err := time.Parse(layout, clean); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}

	// Short DD/MM date common in Brazilian credit card PDF invoices
	if len(clean) == 5 && (clean[2] == '/' || clean[2] == '-') {
		normalized := clean[:2] + "/" + clean[3:]
		if t, err := time.Parse("02/01", normalized); err == nil {
			year := defaultYear
			if year <= 0 {
				year = time.Now().UTC().Year()
			}
			return time.Date(year, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date format %q", raw)
}

func isASCIIAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
