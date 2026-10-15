package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// CreateCategoryRequest defines the payload for creating a budget category.
type CreateCategoryRequest struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	ParentID *string `json:"parent_id,omitempty"`
	ColorHex string  `json:"color_hex,omitempty"`
	IconName string  `json:"icon_name,omitempty"`
	IsSystem *bool   `json:"is_system,omitempty"`
}

// Validate validates input fields of CreateCategoryRequest.
func (r *CreateCategoryRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam
	if strings.TrimSpace(r.Name) == "" {
		errs = append(errs, problem.InvalidParam{Name: "name", Reason: "cannot be empty"})
	}
	txType := domain.TransactionType(strings.ToUpper(strings.TrimSpace(r.Type)))
	if !txType.IsValid() {
		errs = append(errs, problem.InvalidParam{Name: "type", Reason: "must be one of INCOME, EXPENSE, TRANSFER"})
	}
	if r.ParentID != nil && strings.TrimSpace(*r.ParentID) != "" {
		if _, err := uuid.Parse(strings.TrimSpace(*r.ParentID)); err != nil {
			errs = append(errs, problem.InvalidParam{Name: "parent_id", Reason: "must be a valid UUID"})
		}
	}
	return errs
}

// UpdateCategoryRequest defines the payload for updating an existing category.
type UpdateCategoryRequest struct {
	Name     *string `json:"name,omitempty"`
	ParentID *string `json:"parent_id,omitempty"`
	ColorHex *string `json:"color_hex,omitempty"`
	IconName *string `json:"icon_name,omitempty"`
}

// Validate validates input fields of UpdateCategoryRequest.
func (r *UpdateCategoryRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		errs = append(errs, problem.InvalidParam{Name: "name", Reason: "cannot be empty"})
	}
	if r.ParentID != nil && strings.TrimSpace(*r.ParentID) != "" {
		if _, err := uuid.Parse(strings.TrimSpace(*r.ParentID)); err != nil {
			errs = append(errs, problem.InvalidParam{Name: "parent_id", Reason: "must be a valid UUID"})
		}
	}
	return errs
}

// CategoryResponse defines the serializable response for a category, supporting nested hierarchy.
type CategoryResponse struct {
	ID            uuid.UUID          `json:"id"`
	TenantID      uuid.UUID          `json:"tenant_id"`
	ParentID      *uuid.UUID         `json:"parent_id"`
	Name          string             `json:"name"`
	Type          string             `json:"type"`
	ColorHex      string             `json:"color_hex"`
	IconName      string             `json:"icon_name"`
	IsSystem      bool               `json:"is_system"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	Subcategories []CategoryResponse `json:"subcategories,omitempty"`
}

// CategoryListResponse defines the collection envelope for categories.
type CategoryListResponse struct {
	Items []CategoryResponse `json:"items"`
}

// ToCategoryResponse transforms a domain.Category into CategoryResponse without subcategories.
func ToCategoryResponse(c *domain.Category) CategoryResponse {
	return CategoryResponse{
		ID:        c.ID,
		TenantID:  c.TenantID,
		ParentID:  c.ParentID,
		Name:      c.Name,
		Type:      string(c.Type),
		ColorHex:  c.ColorHex,
		IconName:  c.IconName,
		IsSystem:  c.IsSystem,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// ToCategoryHierarchy organizes flat category slices into a hierarchical taxonomy tree.
func ToCategoryHierarchy(categories []*domain.Category) CategoryListResponse {
	nodeMap := make(map[uuid.UUID]*CategoryResponse, len(categories))
	for _, c := range categories {
		resp := ToCategoryResponse(c)
		nodeMap[c.ID] = &resp
	}

	// First pass: attach children to parents in nodeMap
	for _, c := range categories {
		if c.ParentID != nil && *c.ParentID != uuid.Nil {
			parent, exists := nodeMap[*c.ParentID]
			if exists {
				parent.Subcategories = append(parent.Subcategories, *nodeMap[c.ID])
			}
		}
	}

	// Second pass: collect root items (or orphans whose parent is not in the set)
	var rootItems []CategoryResponse
	for _, c := range categories {
		if c.ParentID == nil || *c.ParentID == uuid.Nil {
			rootItems = append(rootItems, *nodeMap[c.ID])
		} else if _, parentExists := nodeMap[*c.ParentID]; !parentExists {
			rootItems = append(rootItems, *nodeMap[c.ID])
		}
	}

	if rootItems == nil {
		rootItems = []CategoryResponse{}
	}
	return CategoryListResponse{Items: rootItems}
}

