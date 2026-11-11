// Command import-ledger previews or applies a reviewed canonical cash source.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/storage"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/bootstrap"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Importação recusada:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("import-ledger", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("file", "", "read-only canonical source")
	accountValue := flags.String("account", "", "existing canonical account UUID")
	archive := flags.String("archive-dir", "", "absolute private archive outside Git")
	name := flags.String("source-name", "", "provenance label; defaults to source basename")
	apply := flags.Bool("apply", false, "commit only after all personal-pilot gates have passed")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid import arguments")
	}
	account, err := uuid.Parse(*accountValue)
	if err != nil || account == uuid.Nil || account.String() != *accountValue {
		return errors.New("canonical account UUID required; no token normalization")
	}
	if *file == "" {
		return errors.New("read-only canonical source path required")
	}
	cfg := config.Load()
	if cfg.Environment != "personal" {
		return errors.New("authenticated isolated personal/QA configuration required")
	}
	if err = cfg.ValidateRuntime(); err != nil {
		return err
	}
	//nolint:gosec // G304: explicit operator-selected local input, never a server-supplied path.
	source, err := os.Open(*file)
	if err != nil {
		return errors.New("cannot open canonical source")
	}
	defer func() { _ = source.Close() }()
	content, err := io.ReadAll(io.LimitReader(source, 15*1024*1024+1))
	if err != nil {
		return errors.New("cannot read canonical source")
	}
	sourceName := *name
	if sourceName == "" {
		sourceName = filepath.Base(*file)
	}
	req := ports.CanonicalImportRequest{TenantID: cfg.PersonalTenantID, UserID: cfg.PersonalUserID, AccountID: account, SourceName: sourceName, Content: content}
	sourceArchive := storage.NewLocalSourceArchive(*archive)
	if *apply {
		if err = sourceArchive.Validate(); err != nil {
			return err
		}
	}
	parser := parsers.NewCanonicalCSVParser()
	rows, err := parser.ParseCanonical(ctx, req)
	if err != nil {
		return err
	}
	if !*apply {
		return json.NewEncoder(out).Encode(map[string]any{"parsed": len(rows), "applied": false, "financial_semantics_approved": false})
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("cannot open protected import database")
	}
	defer pool.Close()
	if err = bootstrap.VerifyPersonalDatabase(ctx, pool, cfg); err != nil {
		return err
	}
	service := app.NewCanonicalIngestionService(parser, postgres.NewTransactionRepository(pool), sourceArchive)
	result, err := service.Ingest(ctx, req)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}
