package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

type SyntheticTransaction struct {
	ID          string          `json:"id"`
	Date        time.Time       `json:"date"`
	Amount      decimal.Decimal `json:"amount"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Account     string          `json:"account"`
	Source      string          `json:"source"`
	FITID       string          `json:"fitid,omitempty"`
}

var brazilianMerchants = []struct {
	Name     string
	Category string
}{
	{"SUPERMERCADO PAO DE ACUCAR", "Alimentação"},
	{"POSTO IPIRANGA", "Transporte"},
	{"DROGARIA SAO PAULO", "Saúde"},
	{"NETFLIX BRASIL", "Assinaturas"},
	{"SPOTIFY RECIFE", "Assinaturas"},
	{"RESTAURANTE MOCOTO", "Alimentação"},
	{"CONDOMINIO RESIDENCIAL", "Moradia"},
	{"ENEL DISTRIBUICAO SP", "Moradia"},
	{"UBER DO BRASIL", "Transporte"},
	{"MERCADO LIVRE", "Lazer"},
	{"AMAZON BRASIL", "Lazer"},
	{"PAG*CLINICAELUMA", "Receita Comercial"},
	{"PADARIA REAL", "Alimentação"},
	{"SABOR DO BRASIL RESTAURANTE", "Alimentação"},
	{"CLARO BRASIL SERVICOS", "Serviços"},
}

func generateSyntheticTransactions(count int, seed int64) []SyntheticTransaction {
	gofakeit.Seed(seed)

	txs := make([]SyntheticTransaction, count)
	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	for i := 0; i < count; i++ {
		var amount decimal.Decimal
		var merchant string
		var category string

		// Every 10th transaction is income
		if i%10 == 0 {
			if i%20 == 0 {
				merchant = "SALARIO MENSAL MULTINACIONAL"
				category = "Salário"
				amount = decimal.NewFromFloat(gofakeit.Float64Range(5000.00, 12000.00)).Round(2)
			} else {
				merchant = "PIX RECEBIDO - CONSULTORIA"
				category = "Receita Comercial"
				amount = decimal.NewFromFloat(gofakeit.Float64Range(1200.00, 4500.00)).Round(2)
			}
		} else {
			mIndex := gofakeit.Number(0, len(brazilianMerchants)-1)
			merchant = brazilianMerchants[mIndex].Name
			category = brazilianMerchants[mIndex].Category
			expenseVal := gofakeit.Float64Range(-850.00, -12.50)
			amount = decimal.NewFromFloat(expenseVal).Round(2)
		}

		txDate := gofakeit.DateRange(startDate, endDate)
		fitid := fmt.Sprintf("2025%02d%02d%05d", txDate.Month(), txDate.Day(), i+1)

		txs[i] = SyntheticTransaction{
			ID:          uuid.New().String(),
			Date:        txDate,
			Amount:      amount,
			Description: merchant,
			Category:    category,
			Account:     "033-12345-6",
			Source:      "ITAU",
			FITID:       fitid,
		}
	}

	return txs
}

func main() {
	countFlag := flag.Int("count", 100, "Number of synthetic transactions to generate")
	outDirFlag := flag.String("out", "test/fixtures", "Output directory for fixture files")
	flag.Parse()

	if err := os.MkdirAll(*outDirFlag, 0755); err != nil {
		log.Fatalf("failed to create output directory %s: %v", *outDirFlag, err)
	}

	txs := generateSyntheticTransactions(*countFlag, 42)

	itauOFXPath := filepath.Join(*outDirFlag, "extrato_itau.ofx")
	if err := exportItauOFX(itauOFXPath, txs); err != nil {
		log.Fatalf("failed to export Itaú OFX: %v", err)
	}

	itauCSVPath := filepath.Join(*outDirFlag, "extrato_itau.csv")
	if err := exportItauCSV(itauCSVPath, txs); err != nil {
		log.Fatalf("failed to export Itaú CSV: %v", err)
	}

	nubankCSVPath := filepath.Join(*outDirFlag, "fatura_nubank.csv")
	if err := exportNubankCSV(nubankCSVPath, txs); err != nil {
		log.Fatalf("failed to export Nubank CSV: %v", err)
	}

	excelPath := filepath.Join(*outDirFlag, "historico_anual.xlsx")
	if err := exportAnnualExcel(excelPath, txs); err != nil {
		log.Fatalf("failed to export Annual XLSX: %v", err)
	}

	jsonPath := filepath.Join(*outDirFlag, "transacoes.json")
	if err := exportJSON(jsonPath, txs); err != nil {
		log.Fatalf("failed to export JSON: %v", err)
	}

	fmt.Printf("Successfully generated %d synthetic transactions into %s:\n", len(txs), *outDirFlag)
	fmt.Printf(" - %s\n", itauOFXPath)
	fmt.Printf(" - %s\n", itauCSVPath)
	fmt.Printf(" - %s\n", nubankCSVPath)
	fmt.Printf(" - %s\n", excelPath)
	fmt.Printf(" - %s\n", jsonPath)
}

func exportItauOFX(path string, txs []SyntheticTransaction) error {
	var buf bytes.Buffer

	// Standard Itaú OFX Header
	buf.WriteString("OFXHEADER:100\n")
	buf.WriteString("DATA:OFXSGML\n")
	buf.WriteString("VERSION:102\n")
	buf.WriteString("SECURITY:NONE\n")
	buf.WriteString("ENCODING:USASCII\n")
	buf.WriteString("CHARSET:1252\n")
	buf.WriteString("COMPRESSION:NONE\n")
	buf.WriteString("OLDFILEUID:NONE\n")
	buf.WriteString("NEWFILEUID:NONE\n\n")

	buf.WriteString("<OFX>\n")
	buf.WriteString("  <SIGNONMSGSRSV1>\n")
	buf.WriteString("    <SONRS>\n")
	buf.WriteString("      <STATUS>\n")
	buf.WriteString("        <CODE>0\n")
	buf.WriteString("        <SEVERITY>INFO\n")
	buf.WriteString("      </STATUS>\n")
	buf.WriteString("      <DTSERVER>20251231235959[-3:BRT]\n")
	buf.WriteString("      <LANGUAGE>POR\n")
	buf.WriteString("      <FI>\n")
	buf.WriteString("        <ORG>Banco Itau SA\n")
	buf.WriteString("        <FID>341\n")
	buf.WriteString("      </FI>\n")
	buf.WriteString("    </SONRS>\n")
	buf.WriteString("  </SIGNONMSGSRSV1>\n")

	buf.WriteString("  <BANKMSGSRSV1>\n")
	buf.WriteString("    <STMTTRNRS>\n")
	buf.WriteString("      <TRNUID>1001\n")
	buf.WriteString("      <STATUS>\n")
	buf.WriteString("        <CODE>0\n")
	buf.WriteString("        <SEVERITY>INFO\n")
	buf.WriteString("      </STATUS>\n")
	buf.WriteString("      <STMTRS>\n")
	buf.WriteString("        <CURDEF>BRL\n")
	buf.WriteString("        <BANKACCTFROM>\n")
	buf.WriteString("          <BANKID>0341\n")
	buf.WriteString("          <ACCTID>033123456\n")
	buf.WriteString("          <ACCTTYPE>CHECKING\n")
	buf.WriteString("        </BANKACCTFROM>\n")
	buf.WriteString("        <BANKTRANLIST>\n")
	buf.WriteString("          <DTSTART>20250101000000[-3:BRT]\n")
	buf.WriteString("          <DTEND>20251231235959[-3:BRT]\n")

	for _, tx := range txs {
		trnType := "OTHER"
		if tx.Amount.IsPositive() {
			trnType = "CREDIT"
		} else {
			trnType = "DEBIT"
		}

		dateStr := tx.Date.Format("20060102") + "120000[-3:BRT]"

		buf.WriteString("          <STMTTRN>\n")
		buf.WriteString(fmt.Sprintf("            <TRNTYPE>%s\n", trnType))
		buf.WriteString(fmt.Sprintf("            <DTPOSTED>%s\n", dateStr))
		buf.WriteString(fmt.Sprintf("            <TRNAMT>%s\n", tx.Amount.StringFixed(2)))
		buf.WriteString(fmt.Sprintf("            <FITID>%s\n", tx.FITID))
		buf.WriteString(fmt.Sprintf("            <MEMO>%s\n", tx.Description))
		buf.WriteString("          </STMTTRN>\n")
	}

	buf.WriteString("        </BANKTRANLIST>\n")
	buf.WriteString("        <LEDGERBAL>\n")
	buf.WriteString("          <BALAMT>15420.50\n")
	buf.WriteString("          <DTASOF>20251231235959[-3:BRT]\n")
	buf.WriteString("        </LEDGERBAL>\n")
	buf.WriteString("      </STMTRS>\n")
	buf.WriteString("    </STMTTRNRS>\n")
	buf.WriteString("  </BANKMSGSRSV1>\n")
	buf.WriteString("</OFX>\n")

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func exportItauCSV(path string, txs []SyntheticTransaction) error {
	var buf bytes.Buffer

	// Itaú CSV Header: Semicolon separator, Brazilian BRL decimal comma
	buf.WriteString("Data;Lancamento;Valor;Categoria;Identificador\n")

	for _, tx := range txs {
		dateStr := tx.Date.Format("02/01/2006")
		amountStr := formatBRLDecimal(tx.Amount.StringFixed(2))
		buf.WriteString(fmt.Sprintf("%s;%s;%s;%s;%s\n", dateStr, tx.Description, amountStr, tx.Category, tx.FITID))
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func formatBRLDecimal(s string) string {
	return strings.ReplaceAll(s, ".", ",")
}

func exportNubankCSV(path string, txs []SyntheticTransaction) error {
	var buf bytes.Buffer

	// Nubank Credit Card Invoice Header: comma separator
	// Format: date,category,title,amount
	// Expenses in Nubank invoice are reported as positive values, invoice payments as negative
	buf.WriteString("date,category,title,amount\n")

	for _, tx := range txs {
		dateStr := tx.Date.Format("2006-01-02")
		// Inverted Nubank credit sign logic
		nubankAmount := tx.Amount.Neg().StringFixed(2)
		buf.WriteString(fmt.Sprintf("%s,%s,%s,%s\n", dateStr, strings.ToLower(tx.Category), tx.Description, nubankAmount))
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func exportAnnualExcel(path string, txs []SyntheticTransaction) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Histórico 2025"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}

	_ = f.DeleteSheet("Sheet1")
	f.SetActiveSheet(index)

	// Set header values
	headers := []string{"Data", "Descrição", "Valor (R$)", "Categoria", "Conta", "FITID"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	// Format header style
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"1F4E78"},
			Pattern: 1,
		},
	})
	if err == nil {
		_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)
	}

	// Fill row data
	for rowIdx, tx := range txs {
		r := rowIdx + 2
		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", r), tx.Date.Format("02/01/2006"))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", r), tx.Description)

		valFloat, _ := tx.Amount.Float64()
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", r), valFloat)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", r), tx.Category)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", r), tx.Account)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", r), tx.FITID)
	}

	return f.SaveAs(path)
}

func exportJSON(path string, txs []SyntheticTransaction) error {
	data, err := json.MarshalIndent(txs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
