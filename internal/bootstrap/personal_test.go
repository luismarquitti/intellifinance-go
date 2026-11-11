package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

type personalRow struct {
	allowed bool
	err     error
}

func (r personalRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*bool)) = r.allowed
	return nil
}

type personalQuery struct{ row personalRow }

func (q personalQuery) QueryRow(context.Context, string, ...any) pgx.Row { return q.row }

func TestPersonalDatabaseProofFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  personalRow
		ok   bool
	}{
		{"missing marker", personalRow{err: errors.New("synthetic missing marker")}, false},
		{"wrong role or identity", personalRow{allowed: false}, false},
		{"proven reserved database", personalRow{allowed: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPersonalDatabase(context.Background(), personalQuery{tc.row}, &config.Config{})
			if (err == nil) != tc.ok {
				t.Fatal("personal database proof did not fail closed")
			}
		})
	}
}
