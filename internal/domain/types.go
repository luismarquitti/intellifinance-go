package domain

// TenantType defines the organizational model of a tenant.
type TenantType string

const (
	// TenantTypeHousehold represents a co-budgeting household (e.g., family or partners).
	TenantTypeHousehold TenantType = "HOUSEHOLD"
	// TenantTypePersonal represents an individual personal finance account.
	TenantTypePersonal TenantType = "PERSONAL"
	// TenantTypeBusiness represents a legal entity or business organization.
	TenantTypeBusiness TenantType = "BUSINESS"
)

// IsValid checks whether the TenantType is supported.
func (t TenantType) IsValid() bool {
	switch t {
	case TenantTypeHousehold, TenantTypePersonal, TenantTypeBusiness:
		return true
	default:
		return false
	}
}

// MemberRole defines role-based access control permissions within a tenant.
type MemberRole string

const (
	// MemberRoleOwner has full control including tenant deletion and billing.
	MemberRoleOwner MemberRole = "OWNER"
	// MemberRoleAdmin can configure accounts, categories, and invite members.
	MemberRoleAdmin MemberRole = "ADMIN"
	// MemberRoleMember can manage own transactions, shared accounts, and view shared data.
	MemberRoleMember MemberRole = "MEMBER"
	// MemberRoleViewer has read-only access to shared resources.
	MemberRoleViewer MemberRole = "VIEWER"
)

// IsValid checks whether the MemberRole is supported.
func (r MemberRole) IsValid() bool {
	switch r {
	case MemberRoleOwner, MemberRoleAdmin, MemberRoleMember, MemberRoleViewer:
		return true
	default:
		return false
	}
}

// AccountType identifies the financial classification of an account.
type AccountType string

const (
	// AccountTypeChecking is a standard liquid checking account (conta corrente).
	AccountTypeChecking AccountType = "CHECKING"
	// AccountTypeCreditCard is a revolving credit line (cartão de crédito).
	AccountTypeCreditCard AccountType = "CREDIT_CARD"
	// AccountTypeSavings is an interest-bearing savings account (poupança).
	AccountTypeSavings AccountType = "SAVINGS"
	// AccountTypeInvestment represents brokerage or fixed/variable income portfolios.
	AccountTypeInvestment AccountType = "INVESTMENT"
	// AccountTypeCash represents physical cash reserves (dinheiro em espécie).
	AccountTypeCash AccountType = "CASH"
)

// IsValid checks whether the AccountType is supported.
func (a AccountType) IsValid() bool {
	switch a {
	case AccountTypeChecking, AccountTypeCreditCard, AccountTypeSavings, AccountTypeInvestment, AccountTypeCash:
		return true
	default:
		return false
	}
}

// Visibility governs multi-user access boundaries within household tenants.
type Visibility string

const (
	// VisibilityShared is visible to all members of the tenant.
	VisibilityShared Visibility = "SHARED"
	// VisibilityPrivate is visible only to the owning or creating user.
	VisibilityPrivate Visibility = "PRIVATE"
	// VisibilityRestricted is visible only to administrators and specific authorized members.
	VisibilityRestricted Visibility = "RESTRICTED"
)

// IsValid checks whether the Visibility is supported.
func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityShared, VisibilityPrivate, VisibilityRestricted:
		return true
	default:
		return false
	}
}

// TransactionType denotes the directional cashflow classification.
type TransactionType string

const (
	// TransactionTypeIncome represents cash inflows (receitas/proventos).
	TransactionTypeIncome TransactionType = "INCOME"
	// TransactionTypeExpense represents cash outflows (despesas/gastos).
	TransactionTypeExpense TransactionType = "EXPENSE"
	// TransactionTypeTransfer represents neutral transfers between accounts (transferências).
	TransactionTypeTransfer TransactionType = "TRANSFER"
)

// IsValid checks whether the TransactionType is supported.
func (t TransactionType) IsValid() bool {
	switch t {
	case TransactionTypeIncome, TransactionTypeExpense, TransactionTypeTransfer:
		return true
	default:
		return false
	}
}

// TransactionStatus tracks the lifecycle and settlement state of a transaction.
type TransactionStatus string

const (
	// TransactionStatusPending denotes an authorized or unposted transaction.
	TransactionStatusPending TransactionStatus = "PENDING"
	// TransactionStatusCompleted denotes a settled and posted transaction.
	TransactionStatusCompleted TransactionStatus = "COMPLETED"
	// TransactionStatusReconciled denotes a transaction verified against an official bank statement.
	TransactionStatusReconciled TransactionStatus = "RECONCILED"
	// TransactionStatusVoid denotes a cancelled, reversed, or invalidated transaction.
	TransactionStatusVoid TransactionStatus = "VOID"
)

// IsValid checks whether the TransactionStatus is supported.
func (s TransactionStatus) IsValid() bool {
	switch s {
	case TransactionStatusPending, TransactionStatusCompleted, TransactionStatusReconciled, TransactionStatusVoid:
		return true
	default:
		return false
	}
}

// AffectsBalance returns true if transactions in this status count toward the account ledger balance.
func (s TransactionStatus) AffectsBalance() bool {
	return s == TransactionStatusCompleted || s == TransactionStatusReconciled
}

// SourceType defines the origin channel of transactions or statements.
type SourceType string

const (
	// SourceTypePluggy represents Open Finance synchronization via Pluggy.ai.
	SourceTypePluggy SourceType = "PLUGGY"
	// SourceTypeOFX represents Open Financial Exchange statement imports.
	SourceTypeOFX SourceType = "OFX"
	// SourceTypeCSV represents Comma-Separated Values statement imports.
	SourceTypeCSV SourceType = "CSV"
	// SourceTypeExcel represents Microsoft Excel (.xlsx) spreadsheet imports.
	SourceTypeExcel SourceType = "EXCEL"
	// SourceTypePDF represents PDF statement or salary receipt imports.
	SourceTypePDF SourceType = "PDF"
	// SourceTypeManual represents user-entered manual journal records.
	SourceTypeManual SourceType = "MANUAL"
)

// IsValid checks whether the SourceType is supported.
func (s SourceType) IsValid() bool {
	switch s {
	case SourceTypePluggy, SourceTypeOFX, SourceTypeCSV, SourceTypeExcel, SourceTypePDF, SourceTypeManual:
		return true
	default:
		return false
	}
}

// JobStatus tracks asynchronous ingestion job execution states.
type JobStatus string

const (
	// JobStatusPending indicates the job is queued awaiting worker pickup.
	JobStatusPending JobStatus = "PENDING"
	// JobStatusProcessing indicates a worker is actively parsing or categorizing the statement.
	JobStatusProcessing JobStatus = "PROCESSING"
	// JobStatusCompleted indicates the statement was successfully parsed, deduplicated, and staged.
	JobStatusCompleted JobStatus = "COMPLETED"
	// JobStatusFailed indicates unrecoverable failure during ingestion.
	JobStatusFailed JobStatus = "FAILED"
)

// IsValid checks whether the JobStatus is supported.
func (j JobStatus) IsValid() bool {
	switch j {
	case JobStatusPending, JobStatusProcessing, JobStatusCompleted, JobStatusFailed:
		return true
	default:
		return false
	}
}
