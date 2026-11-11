package domain

import "errors"

var (
	// ErrImmutableImportedTransaction rejects changes to retained statement facts.
	ErrImmutableImportedTransaction = errors.New("imported source facts and identity are immutable")
	// ErrReservedProvenance reserves original-source evidence to canonical ingestion.
	ErrReservedProvenance = errors.New("source provenance is reserved for canonical ingestion")
)
