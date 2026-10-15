package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestNewCategory_Success(t *testing.T) {
	tenantID := uuid.New()

	// Root category
	cat, err := domain.NewCategory(
		tenantID,
		nil,
		"Alimentação",
		domain.TransactionTypeExpense,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cat.ID == uuid.Nil {
		t.Error("expected valid category ID")
	}
	if cat.IsSubcategory() {
		t.Error("expected root category not to be subcategory")
	}

	// Subcategory
	subcat, err := domain.NewCategory(
		tenantID,
		&cat.ID,
		"Supermercado",
		domain.TransactionTypeExpense,
		false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !subcat.IsSubcategory() {
		t.Error("expected subcategory to return true for IsSubcategory")
	}
	if *subcat.ParentID != cat.ID {
		t.Errorf("expected parent ID %v, got %v", cat.ID, *subcat.ParentID)
	}
}

func TestNewCategory_ValidationErrors(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name        string
		tenantID    uuid.UUID
		parentID    *uuid.UUID
		catName     string
		catType     domain.TransactionType
		isSystem    bool
		expectedErr error
	}{
		{
			name:        "Nil tenant ID",
			tenantID:    uuid.Nil,
			parentID:    nil,
			catName:     "Lazer",
			catType:     domain.TransactionTypeExpense,
			expectedErr: domain.ErrInvalidID,
		},
		{
			name:        "Empty category name",
			tenantID:    tenantID,
			parentID:    nil,
			catName:     "   ",
			catType:     domain.TransactionTypeExpense,
			expectedErr: domain.ErrEmptyName,
		},
		{
			name:        "Invalid transaction type",
			tenantID:    tenantID,
			parentID:    nil,
			catName:     "Lazer",
			catType:     domain.TransactionType("INVALID"),
			expectedErr: domain.ErrInvalidTransactionType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewCategory(
				tc.tenantID,
				tc.parentID,
				tc.catName,
				tc.catType,
				tc.isSystem,
			)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestCategory_Validate_SelfReferenceParent(t *testing.T) {
	catID := uuid.New()
	tenantID := uuid.New()

	cat := &domain.Category{
		ID:       catID,
		TenantID: tenantID,
		ParentID: &catID, // Self loop
		Name:     "Recursion",
		Type:     domain.TransactionTypeExpense,
	}

	if err := cat.Validate(); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("expected ErrInvalidID on self-referencing parent, got %v", err)
	}
}
