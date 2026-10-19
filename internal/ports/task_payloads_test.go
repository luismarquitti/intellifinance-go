package ports_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

type validatable interface {
	Validate() error
}

func payloadFixtures(t *testing.T) map[ports.TaskType]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile("../../test/fixtures/task_payloads.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[ports.TaskType]json.RawMessage
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestTaskPayloadRoundTrip(t *testing.T) {
	fixtures := payloadFixtures(t)
	for taskType, payload := range map[ports.TaskType]validatable{
		ports.TaskTypeIngestStatement:       &ports.IngestStatementPayload{},
		ports.TaskTypeSyncOpenFinance:       &ports.SyncOpenFinancePayload{},
		ports.TaskTypeCategorizeTransaction: &ports.CategorizeTransactionPayload{},
	} {
		t.Run(string(taskType), func(t *testing.T) {
			if err := json.Unmarshal(fixtures[taskType], payload); err != nil {
				t.Fatal(err)
			}
			if err := payload.Validate(); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			var original, roundTrip map[string]json.RawMessage
			if err := json.Unmarshal(fixtures[taskType], &original); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatal(err)
			}
			// Compact values so whitespace in the fixture cannot mask wire changes.
			for key, value := range original {
				var want, got any
				if err := json.Unmarshal(value, &want); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(roundTrip[key], &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(want, got) {
					t.Errorf("field %s changed: want %v, got %v", key, want, got)
				}
			}
		})
	}
}

func TestIngestStatementPayloadValidation(t *testing.T) {
	var valid ports.IngestStatementPayload
	if err := json.Unmarshal(payloadFixtures(t)[ports.TaskTypeIngestStatement], &valid); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*ports.IngestStatementPayload){
		"tenant":             func(p *ports.IngestStatementPayload) { p.TenantID = uuid.Nil },
		"job":                func(p *ports.IngestStatementPayload) { p.JobID = uuid.Nil },
		"account":            func(p *ports.IngestStatementPayload) { p.AccountID = uuid.Nil },
		"user":               func(p *ports.IngestStatementPayload) { p.UserID = uuid.Nil },
		"storage":            func(p *ports.IngestStatementPayload) { p.StorageURI = "" },
		"malformed URI":      func(p *ports.IngestStatementPayload) { p.StorageURI = "%zz" },
		"unsupported scheme": func(p *ports.IngestStatementPayload) { p.StorageURI = "https://example.test/a" },
		"empty object":       func(p *ports.IngestStatementPayload) { p.StorageURI = "gs://bucket/" },
		"empty bucket":       func(p *ports.IngestStatementPayload) { p.StorageURI = "s3:///object" },
		"relative file":      func(p *ports.IngestStatementPayload) { p.StorageURI = "file:relative.ofx" },
		"credentials":        func(p *ports.IngestStatementPayload) { p.StorageURI = "s3://user:secret@bucket/a" },
		"filename":           func(p *ports.IngestStatementPayload) { p.SourceFilename = "  " },
		"uploaded at":        func(p *ports.IngestStatementPayload) { p.UploadedAt = time.Time{} },
		"file type":          func(p *ports.IngestStatementPayload) { p.FileType = "UNKNOWN" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			payload := valid
			mutate(&payload)
			err := payload.Validate()
			if !errors.Is(err, ports.ErrInvalidTaskPayload) {
				t.Fatalf("expected invalid task, got %v", err)
			}
			if name == "tenant" && !errors.Is(err, ports.ErrMissingTenantID) {
				t.Fatalf("expected missing tenant, got %v", err)
			}
		})
	}
	for _, uri := range []string{"gs://synthetic-bucket/a", "s3://synthetic-bucket/a", "file:///tmp/synthetic.ofx"} {
		payload := valid
		payload.StorageURI = uri
		if err := payload.Validate(); err != nil {
			t.Errorf("valid storage URI %s: %v", uri, err)
		}
	}
	for _, format := range []ports.SourceFormat{ports.SourceFormatOFX, ports.SourceFormatCSV, ports.SourceFormatExcel, ports.SourceFormatPDF, "XLSX"} {
		payload := valid
		payload.FileType = format
		if err := payload.Validate(); err != nil {
			t.Errorf("valid file format %s: %v", format, err)
		}
	}
}

func TestSyncOpenFinancePayloadValidation(t *testing.T) {
	var valid ports.SyncOpenFinancePayload
	if err := json.Unmarshal(payloadFixtures(t)[ports.TaskTypeSyncOpenFinance], &valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ports.SyncOpenFinancePayload){
		"tenant":   func(p *ports.SyncOpenFinancePayload) { p.TenantID = uuid.Nil },
		"item":     func(p *ports.SyncOpenFinancePayload) { p.ItemID = uuid.Nil },
		"mode":     func(p *ports.SyncOpenFinancePayload) { p.SyncType = "UNKNOWN" },
		"from":     func(p *ports.SyncOpenFinancePayload) { p.FromDate = "2026-02-30" },
		"to":       func(p *ports.SyncOpenFinancePayload) { p.ToDate = "" },
		"reversed": func(p *ports.SyncOpenFinancePayload) { p.FromDate = "2026-01-03" },
		"trigger":  func(p *ports.SyncOpenFinancePayload) { p.TriggeredBy = " " },
	} {
		t.Run(name, func(t *testing.T) {
			payload := valid
			mutate(&payload)
			if err := payload.Validate(); !errors.Is(err, ports.ErrInvalidTaskPayload) {
				t.Fatalf("expected permanent validation error, got %v", err)
			}
		})
	}
	valid.SyncType = "FULL"
	valid.ToDate = valid.FromDate
	if err := valid.Validate(); err != nil {
		t.Fatalf("same-day FULL sync is valid: %v", err)
	}
}

func TestCategorizeTransactionPayloadValidation(t *testing.T) {
	var valid ports.CategorizeTransactionPayload
	if err := json.Unmarshal(payloadFixtures(t)[ports.TaskTypeCategorizeTransaction], &valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ports.CategorizeTransactionPayload){
		"tenant":      func(p *ports.CategorizeTransactionPayload) { p.TenantID = uuid.Nil },
		"empty batch": func(p *ports.CategorizeTransactionPayload) { p.TransactionIDs = nil },
		"nil ID": func(p *ports.CategorizeTransactionPayload) {
			p.TransactionIDs = []uuid.UUID{valid.TransactionIDs[0], uuid.Nil}
		},
	} {
		t.Run(name, func(t *testing.T) {
			payload := valid
			mutate(&payload)
			if err := payload.Validate(); !errors.Is(err, ports.ErrInvalidTaskPayload) {
				t.Fatalf("expected permanent validation error, got %v", err)
			}
		})
	}
	valid.ModelPreference = ""
	if err := valid.Validate(); err != nil {
		t.Fatalf("model preference is optional: %v", err)
	}
}

func TestTaskPayloadsContainNoFloatingPointFields(t *testing.T) {
	var check func(reflect.Type)
	check = func(typ reflect.Type) {
		switch typ.Kind() {
		case reflect.Float32, reflect.Float64:
			t.Errorf("floating point type in task contract: %v", typ)
		case reflect.Pointer, reflect.Slice, reflect.Array:
			check(typ.Elem())
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				if typ.Field(i).IsExported() {
					check(typ.Field(i).Type)
				}
			}
		}
	}
	check(reflect.TypeOf(ports.IngestStatementPayload{}))
	check(reflect.TypeOf(ports.SyncOpenFinancePayload{}))
	check(reflect.TypeOf(ports.CategorizeTransactionPayload{}))
}
