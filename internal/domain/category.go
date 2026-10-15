package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Category represents a hierarchical budget taxonomy node scoped to a tenant.
type Category struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	ParentID  *uuid.UUID      `json:"parent_id,omitempty"`
	Name      string          `json:"name"`
	Type      TransactionType `json:"type"`
	ColorHex  string          `json:"color_hex"`
	IconName  string          `json:"icon_name"`
	IsSystem  bool            `json:"is_system"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// NewCategory constructs and validates a new Category entity.
func NewCategory(
	tenantID uuid.UUID,
	parentID *uuid.UUID,
	name string,
	catType TransactionType,
	isSystem bool,
) (*Category, error) {
	now := time.Now().UTC()
	c := &Category{
		ID:        uuid.New(),
		TenantID:  tenantID,
		ParentID:  parentID,
		Name:      strings.TrimSpace(name),
		Type:      catType,
		ColorHex:  "#64748B",
		IconName:  "tag",
		IsSystem:  isSystem,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate ensures category invariants are satisfied.
func (c *Category) Validate() error {
	if c.ID == uuid.Nil || c.TenantID == uuid.Nil {
		return ErrInvalidID
	}
	if strings.TrimSpace(c.Name) == "" {
		return ErrEmptyName
	}
	if !c.Type.IsValid() {
		return ErrInvalidTransactionType
	}
	if c.ParentID != nil && *c.ParentID == c.ID {
		return ErrInvalidID
	}
	return nil
}

// IsSubcategory returns true if this category has a parent category.
func (c *Category) IsSubcategory() bool {
	return c.ParentID != nil && *c.ParentID != uuid.Nil
}
