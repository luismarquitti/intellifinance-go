package domain_test

import (
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestTenantType_IsValid(t *testing.T) {
	tests := []struct {
		tt    domain.TenantType
		valid bool
	}{
		{domain.TenantTypeHousehold, true},
		{domain.TenantTypePersonal, true},
		{domain.TenantTypeBusiness, true},
		{domain.TenantType("INVALID"), false},
		{domain.TenantType(""), false},
	}

	for _, tt := range tests {
		if got := tt.tt.IsValid(); got != tt.valid {
			t.Errorf("TenantType(%q).IsValid() = %v; want %v", tt.tt, got, tt.valid)
		}
	}
}

func TestMemberRole_IsValid(t *testing.T) {
	tests := []struct {
		role  domain.MemberRole
		valid bool
	}{
		{domain.MemberRoleOwner, true},
		{domain.MemberRoleAdmin, true},
		{domain.MemberRoleMember, true},
		{domain.MemberRoleViewer, true},
		{domain.MemberRole("GUEST"), false},
		{domain.MemberRole(""), false},
	}

	for _, tt := range tests {
		if got := tt.role.IsValid(); got != tt.valid {
			t.Errorf("MemberRole(%q).IsValid() = %v; want %v", tt.role, got, tt.valid)
		}
	}
}

func TestAccountType_IsValid(t *testing.T) {
	tests := []struct {
		at    domain.AccountType
		valid bool
	}{
		{domain.AccountTypeChecking, true},
		{domain.AccountTypeCreditCard, true},
		{domain.AccountTypeSavings, true},
		{domain.AccountTypeInvestment, true},
		{domain.AccountTypeCash, true},
		{domain.AccountType("CRYPTO"), false},
		{domain.AccountType(""), false},
	}

	for _, tt := range tests {
		if got := tt.at.IsValid(); got != tt.valid {
			t.Errorf("AccountType(%q).IsValid() = %v; want %v", tt.at, got, tt.valid)
		}
	}
}

func TestVisibility_IsValid(t *testing.T) {
	tests := []struct {
		vis   domain.Visibility
		valid bool
	}{
		{domain.VisibilityShared, true},
		{domain.VisibilityPrivate, true},
		{domain.VisibilityRestricted, true},
		{domain.Visibility("HIDDEN"), false},
		{domain.Visibility(""), false},
	}

	for _, tt := range tests {
		if got := tt.vis.IsValid(); got != tt.valid {
			t.Errorf("Visibility(%q).IsValid() = %v; want %v", tt.vis, got, tt.valid)
		}
	}
}

func TestTransactionType_IsValid(t *testing.T) {
	tests := []struct {
		txt   domain.TransactionType
		valid bool
	}{
		{domain.TransactionTypeIncome, true},
		{domain.TransactionTypeExpense, true},
		{domain.TransactionTypeTransfer, true},
		{domain.TransactionType("REFUND"), false},
		{domain.TransactionType(""), false},
	}

	for _, tt := range tests {
		if got := tt.txt.IsValid(); got != tt.valid {
			t.Errorf("TransactionType(%q).IsValid() = %v; want %v", tt.txt, got, tt.valid)
		}
	}
}

func TestTransactionStatus_IsValidAndAffectsBalance(t *testing.T) {
	tests := []struct {
		status         domain.TransactionStatus
		valid          bool
		affectsBalance bool
	}{
		{domain.TransactionStatusPending, true, false},
		{domain.TransactionStatusCompleted, true, true},
		{domain.TransactionStatusReconciled, true, true},
		{domain.TransactionStatusVoid, true, false},
		{domain.TransactionStatus("ARCHIVED"), false, false},
		{domain.TransactionStatus(""), false, false},
	}

	for _, tt := range tests {
		if got := tt.status.IsValid(); got != tt.valid {
			t.Errorf("TransactionStatus(%q).IsValid() = %v; want %v", tt.status, got, tt.valid)
		}
		if got := tt.status.AffectsBalance(); got != tt.affectsBalance {
			t.Errorf("TransactionStatus(%q).AffectsBalance() = %v; want %v", tt.status, got, tt.affectsBalance)
		}
	}
}

func TestSourceType_IsValid(t *testing.T) {
	tests := []struct {
		source domain.SourceType
		valid  bool
	}{
		{domain.SourceTypePluggy, true},
		{domain.SourceTypeOFX, true},
		{domain.SourceTypeCSV, true},
		{domain.SourceTypeExcel, true},
		{domain.SourceTypePDF, true},
		{domain.SourceTypeManual, true},
		{domain.SourceType("WEBHOOK"), false},
		{domain.SourceType(""), false},
	}

	for _, tt := range tests {
		if got := tt.source.IsValid(); got != tt.valid {
			t.Errorf("SourceType(%q).IsValid() = %v; want %v", tt.source, got, tt.valid)
		}
	}
}

func TestJobStatus_IsValid(t *testing.T) {
	tests := []struct {
		js    domain.JobStatus
		valid bool
	}{
		{domain.JobStatusPending, true},
		{domain.JobStatusProcessing, true},
		{domain.JobStatusCompleted, true},
		{domain.JobStatusFailed, true},
		{domain.JobStatus("TIMEOUT"), false},
		{domain.JobStatus(""), false},
	}

	for _, tt := range tests {
		if got := tt.js.IsValid(); got != tt.valid {
			t.Errorf("JobStatus(%q).IsValid() = %v; want %v", tt.js, got, tt.valid)
		}
	}
}
