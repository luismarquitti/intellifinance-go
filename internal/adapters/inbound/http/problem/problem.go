package problem

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// ContentTypeProblemJSON defines the MIME media type for RFC 7807 problem details.
const ContentTypeProblemJSON = "application/problem+json"

// Type URIs for standard API error categories.
const (
	TypeValidationFailed = "https://api.intellifinance.app/errors/validation-failed"
	TypeNotFound         = "https://api.intellifinance.app/errors/not-found"
	TypeBadRequest       = "https://api.intellifinance.app/errors/bad-request"
	TypeUnauthorized     = "https://api.intellifinance.app/errors/unauthorized"
	TypeForbidden        = "https://api.intellifinance.app/errors/forbidden"
	TypeConflict         = "https://api.intellifinance.app/errors/conflict"
	TypeInternalError    = "https://api.intellifinance.app/errors/internal-error"
)

// Error codes for machine readability.
const (
	CodeValidationFailed     = "ERR_VALIDATION_FAILED"
	CodeNotFound             = "ERR_NOT_FOUND"
	CodeBadRequest           = "ERR_BAD_REQUEST"
	CodeUnauthorized         = "ERR_UNAUTHORIZED"
	CodeForbidden            = "ERR_FORBIDDEN"
	CodeConflict             = "ERR_CONFLICT"
	CodeDuplicateTransaction = "ERR_DUPLICATE_TRANSACTION"
	CodeInternalError        = "ERR_INTERNAL_ERROR"
)

// InvalidParam describes a specific payload or parameter validation error.
type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Detail represents an RFC 7807 compliant problem details response.
type Detail struct {
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Status        int            `json:"status"`
	Detail        string         `json:"detail"`
	Instance      string         `json:"instance,omitempty"`
	Code          string         `json:"code,omitempty"`
	Timestamp     string         `json:"timestamp"`
	InvalidParams []InvalidParam `json:"invalid_params,omitempty"`
}

// ProblemDetail is an alias for Detail to maintain RFC 7807 naming compatibility.
//nolint:revive // Alias preserves RFC 7807 naming
type ProblemDetail = Detail

// New constructs a base Detail.
func New(status int, typeURI, title, detail, instance, code string) Detail {
	return Detail{
		Type:      typeURI,
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  instance,
		Code:      code,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewValidationFailed constructs an RFC 7807 problem for validation failures (HTTP 422).
func NewValidationFailed(instance, detail string, invalidParams []InvalidParam) Detail {
	p := New(
		http.StatusUnprocessableEntity,
		TypeValidationFailed,
		"Validation Failed",
		detail,
		instance,
		CodeValidationFailed,
	)
	p.InvalidParams = invalidParams
	return p
}

// NewNotFound constructs an RFC 7807 problem for missing resources (HTTP 404).
func NewNotFound(instance, detail string) Detail {
	return New(
		http.StatusNotFound,
		TypeNotFound,
		"Resource Not Found",
		detail,
		instance,
		CodeNotFound,
	)
}

// NewBadRequest constructs an RFC 7807 problem for malformed syntax or bad parameters (HTTP 400).
func NewBadRequest(instance, detail string) Detail {
	return New(
		http.StatusBadRequest,
		TypeBadRequest,
		"Bad Request",
		detail,
		instance,
		CodeBadRequest,
	)
}

// NewUnauthorized constructs an RFC 7807 problem for missing/invalid credentials or tenant context (HTTP 401).
func NewUnauthorized(instance, detail string) Detail {
	return New(
		http.StatusUnauthorized,
		TypeUnauthorized,
		"Unauthorized",
		detail,
		instance,
		CodeUnauthorized,
	)
}

// NewConflict constructs an RFC 7807 problem for conflict states (HTTP 409).
func NewConflict(instance, detail, code string) Detail {
	if code == "" {
		code = CodeConflict
	}
	return New(
		http.StatusConflict,
		TypeConflict,
		"Conflict",
		detail,
		instance,
		code,
	)
}

// NewInternalError constructs an RFC 7807 problem for internal server errors (HTTP 500).
func NewInternalError(instance, detail string) Detail {
	if detail == "" {
		detail = "An unexpected internal server error occurred."
	}
	return New(
		http.StatusInternalServerError,
		TypeInternalError,
		"Internal Server Error",
		detail,
		instance,
		CodeInternalError,
	)
}

// FromDomainError maps a domain error to an appropriate RFC 7807 Detail.
func FromDomainError(instance string, err error) Detail {

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return NewNotFound(instance, err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		return NewUnauthorized(instance, err.Error())
	case errors.Is(err, domain.ErrDuplicateTransaction):
		return NewConflict(instance, err.Error(), CodeDuplicateTransaction)
	case errors.Is(err, domain.ErrInvalidAmount):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "amount", Reason: "must be a non-zero signed decimal value matching transaction type"},
		})
	case errors.Is(err, domain.ErrEmptyDescription):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "description", Reason: "cannot be empty or whitespace only"},
		})
	case errors.Is(err, domain.ErrEmptyName):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "name", Reason: "cannot be empty or whitespace only"},
		})
	case errors.Is(err, domain.ErrInvalidEmail):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "email", Reason: "must be a valid email address"},
		})
	case errors.Is(err, domain.ErrInvalidDate):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "date", Reason: "must be a valid date"},
		})
	case errors.Is(err, domain.ErrInvalidAccountType):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "type", Reason: "must be one of CHECKING, CREDIT_CARD, SAVINGS, INVESTMENT, CASH"},
		})
	case errors.Is(err, domain.ErrInvalidTransactionType):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "type", Reason: "must be one of INCOME, EXPENSE, TRANSFER"},
		})
	case errors.Is(err, domain.ErrInvalidVisibility):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "visibility", Reason: "must be one of SHARED, PRIVATE, RESTRICTED"},
		})
	case errors.Is(err, domain.ErrInvalidCurrency):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "currency", Reason: "must be a valid 3-letter ISO 4217 currency code"},
		})
	case errors.Is(err, domain.ErrInvalidSplit):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "splits", Reason: "split allocations must sum exactly to 100% or transaction total"},
		})
	case errors.Is(err, domain.ErrInvalidStatus):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "status", Reason: "must be a valid transaction status"},
		})
	case errors.Is(err, domain.ErrInvalidSource):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "source", Reason: "must be a valid source type"},
		})
	case errors.Is(err, domain.ErrInvalidID):
		return NewValidationFailed(instance, err.Error(), []InvalidParam{
			{Name: "id", Reason: "must be a valid non-nil UUID"},
		})
	default:
		return NewInternalError(instance, "An unexpected error occurred while processing the request.")
	}
}

// Render serializes the problem details into JSON and writes it to w with status and Content-Type.
func Render(w http.ResponseWriter, r *http.Request, prob Detail) {
	if prob.Instance == "" && r != nil {
		prob.Instance = r.URL.Path
	}
	if prob.Timestamp == "" {
		prob.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	w.Header().Set("Content-Type", ContentTypeProblemJSON)
	w.WriteHeader(prob.Status)
	_ = json.NewEncoder(w).Encode(prob)
}
