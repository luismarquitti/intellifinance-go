package domain

import "errors"

// Sentinel domain errors representing common business rule violations and failure states.
var (
	// ErrNotFound is returned when an entity or resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrUnauthorized is returned when an action violates authorization or tenant boundaries.
	ErrUnauthorized = errors.New("unauthorized access")

	// ErrDuplicateTransaction is returned when a transaction fingerprint matches an existing record.
	ErrDuplicateTransaction = errors.New("duplicate transaction detected")

	// ErrInvalidAmount is returned when a monetary amount is zero, non-numeric, or mathematically invalid.
	ErrInvalidAmount = errors.New("invalid monetary amount")

	// ErrEmptyDescription is returned when a transaction description is blank or whitespace-only.
	ErrEmptyDescription = errors.New("description cannot be empty")

	// ErrEmptyName is returned when an entity name (account, category, user, tenant) is empty.
	ErrEmptyName = errors.New("name cannot be empty")

	// ErrInvalidEmail is returned when an email address does not conform to standard format.
	ErrInvalidEmail = errors.New("invalid email address")

	// ErrInvalidDate is returned when a transaction date is zero or invalid.
	ErrInvalidDate = errors.New("invalid transaction date")

	// ErrInvalidAccountType is returned when an account type is not supported.
	ErrInvalidAccountType = errors.New("invalid account type")

	// ErrInvalidTransactionType is returned when a transaction type is not supported.
	ErrInvalidTransactionType = errors.New("invalid transaction type")

	// ErrInvalidVisibility is returned when an access visibility level is not supported.
	ErrInvalidVisibility = errors.New("invalid visibility")

	// ErrInvalidCurrency is returned when a currency code is not a valid 3-letter ISO 4217 code.
	ErrInvalidCurrency = errors.New("invalid currency code")

	// ErrInvalidSplit is returned when transaction splits violate business rules (sum mismatch or ratio).
	ErrInvalidSplit = errors.New("invalid transaction split")

	// ErrInvalidStatus is returned when an entity status transition or value is invalid.
	ErrInvalidStatus = errors.New("invalid status")

	// ErrInvalidSource is returned when a source type is unsupported.
	ErrInvalidSource = errors.New("invalid source type")

	// ErrInvalidTenantType is returned when a tenant type is unsupported.
	ErrInvalidTenantType = errors.New("invalid tenant type")

	// ErrInvalidMemberRole is returned when a tenant member role is unsupported.
	ErrInvalidMemberRole = errors.New("invalid member role")

	// ErrInvalidID is returned when a required UUID is nil or uninitialized.
	ErrInvalidID = errors.New("invalid identifier")
)
