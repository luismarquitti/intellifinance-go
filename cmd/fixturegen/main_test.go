package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestFixtureGenExporters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fixturegen_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txs := generateSyntheticTransactions(10, 12345)
	if len(txs) != 10 {
		t.Fatalf("expected 10 synthetic transactions, got %d", len(txs))
	}

	// 1. Test Itaú OFX
	ofxPath := filepath.Join(tmpDir, "extrato_itau.ofx")
	if err := exportItauOFX(ofxPath, txs); err != nil {
		t.Fatalf("exportItauOFX failed: %v", err)
	}
	ofxData, err := os.ReadFile(ofxPath)
	if err != nil {
		t.Fatalf("failed to read generated OFX: %v", err)
	}
	ofxStr := string(ofxData)
	if !strings.Contains(ofxStr, "<OFX>") || !strings.Contains(ofxStr, "<STMTTRN>") {
		t.Errorf("OFX file missing standard tags")
	}

	// 2. Test Itaú CSV
	itauCSVPath := filepath.Join(tmpDir, "extrato_itau.csv")
	if err := exportItauCSV(itauCSVPath, txs); err != nil {
		t.Fatalf("exportItauCSV failed: %v", err)
	}
	itauCSVData, err := os.ReadFile(itauCSVPath)
	if err != nil {
		t.Fatalf("failed to read Itaú CSV: %v", err)
	}
	if !strings.HasPrefix(string(itauCSVData), "Data;Lancamento;Valor;Categoria;Identificador") {
		t.Errorf("Itaú CSV missing expected header")
	}

	// 3. Test Nubank CSV
	nubankCSVPath := filepath.Join(tmpDir, "fatura_nubank.csv")
	if err := exportNubankCSV(nubankCSVPath, txs); err != nil {
		t.Fatalf("exportNubankCSV failed: %v", err)
	}
	nubankCSVData, err := os.ReadFile(nubankCSVPath)
	if err != nil {
		t.Fatalf("failed to read Nubank CSV: %v", err)
	}
	if !strings.HasPrefix(string(nubankCSVData), "date,category,title,amount") {
		t.Errorf("Nubank CSV missing expected header")
	}

	// 4. Test Annual XLSX
	xlsxPath := filepath.Join(tmpDir, "historico_anual.xlsx")
	if err := exportAnnualExcel(xlsxPath, txs); err != nil {
		t.Fatalf("exportAnnualExcel failed: %v", err)
	}
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatalf("failed to open generated XLSX: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Histórico 2025")
	if err != nil {
		t.Fatalf("failed to read sheet rows: %v", err)
	}
	if len(rows) != 11 { // 1 header + 10 data rows
		t.Errorf("expected 11 rows in XLSX, got %d", len(rows))
	}

	// 5. Test JSON Export
	jsonPath := filepath.Join(tmpDir, "transacoes.json")
	if err := exportJSON(jsonPath, txs); err != nil {
		t.Fatalf("exportJSON failed: %v", err)
	}
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read generated JSON: %v", err)
	}
	var readTxs []SyntheticTransaction
	if err := json.Unmarshal(jsonData, &readTxs); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(readTxs) != 10 {
		t.Errorf("expected 10 transactions in JSON, got %d", len(readTxs))
	}
}
