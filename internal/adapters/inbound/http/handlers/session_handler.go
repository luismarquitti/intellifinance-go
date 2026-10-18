package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

// SessionHandler serves metadata about the active tenant and user session.
type SessionHandler struct {
	cfg *config.Config
}

// NewSessionHandler creates a new SessionHandler instance.
func NewSessionHandler(cfg *config.Config) *SessionHandler {
	return &SessionHandler{cfg: cfg}
}

// SessionResponse represents the active session profile for the client.
type SessionResponse struct {
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id"`
	TenantName  string `json:"tenant_name"`
	UserName    string `json:"user_name"`
	UserEmail   string `json:"user_email"`
	Environment string `json:"environment"`
}

// GetSession handles GET /session.
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		problem.Render(w, r, problem.NewUnauthorized(r.URL.Path, "Missing tenant context."))
		return
	}

	userID, _ := middleware.UserIDFromContext(r.Context())

	resp := SessionResponse{
		TenantID:    tenantID.String(),
		UserID:      userID.String(),
		TenantName:  h.cfg.DevTenantName,
		UserName:    h.cfg.DevAdminName,
		UserEmail:   h.cfg.DevAdminEmail,
		Environment: h.cfg.Environment,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
