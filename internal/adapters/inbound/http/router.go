package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// RouterConfig contains dependencies and configuration options for assembling the API router.
type RouterConfig struct {
	AccountService            ports.AccountService
	CategoryService           ports.CategoryService
	TransactionService        ports.TransactionService
	TransferService           ports.TransferService
	CSVIngestionService       ports.CSVIngestionService
	OpenFinanceWebhookHandler *handlers.OpenFinanceWebhookHandler
	OpenFinanceTenantResolver ports.OpenFinanceTenantResolver
	TaskQueue                 ports.TaskQueue
	OpenFinanceWebhookSecret  string
	AppConfig                 *config.Config
	Logger                    *slog.Logger
	CORSOptions               *cors.Options
	ReadinessChecker          ports.ReadinessChecker
	PendingActionRepo         ports.PendingActionRepository
}

// NewRouter builds and configures the Chi v5 router with all required middlewares,
// health check endpoints, and protected REST routes for accounts, categories, and transactions.
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// 1. Global standard middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger(cfg.Logger))
	r.Use(middleware.Recoverer(cfg.Logger))
	if cfg.AppConfig != nil && cfg.AppConfig.Environment == "personal" {
		r.Use(middleware.PersonalAuthentication(cfg.AppConfig))
	}

	if cfg.CORSOptions != nil {
		r.Use(middleware.CORS(*cfg.CORSOptions))
	} else {
		r.Use(middleware.CORS())
	}

	// 2. Public health check endpoints
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if cfg.ReadinessChecker == nil || cfg.ReadinessChecker.Ping(ctx) != nil {
			problem.Render(w, r, problem.New(http.StatusServiceUnavailable, "about:blank",
				"Service Unavailable", "PostgreSQL is unavailable.", r.URL.Path, "ERR_NOT_READY"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "UP", "checks": map[string]string{"postgres": "UP"},
		})
	})

	// Handlers initialization
	accountHandler := handlers.NewAccountHandler(cfg.AccountService)
	categoryHandler := handlers.NewCategoryHandler(cfg.CategoryService)
	transactionHandler := handlers.NewTransactionHandler(cfg.TransactionService)
	var transferHandler *handlers.TransferHandler
	if cfg.TransferService != nil {
		transferHandler = handlers.NewTransferHandler(cfg.TransferService)
	}
	var csvHandler *handlers.CSVIngestHandler
	if cfg.CSVIngestionService != nil {
		csvHandler = handlers.NewCSVIngestHandler(cfg.CSVIngestionService)
	}
	var pendingActionHandler *handlers.PendingActionHandler
	if cfg.PendingActionRepo != nil {
		pendingActionHandler = handlers.NewPendingActionHandler(cfg.PendingActionRepo, cfg.TransactionService)
	}
	var sessionHandler *handlers.SessionHandler
	if cfg.AppConfig != nil {
		sessionHandler = handlers.NewSessionHandler(cfg.AppConfig)
	}

	openFinanceHandler := cfg.OpenFinanceWebhookHandler
	if openFinanceHandler == nil && cfg.OpenFinanceTenantResolver != nil && cfg.TaskQueue != nil {
		openFinanceHandler = handlers.NewOpenFinanceWebhookHandler(
			cfg.OpenFinanceWebhookSecret,
			cfg.OpenFinanceTenantResolver,
			cfg.TaskQueue,
			cfg.Logger,
		)
	}

	// 3. Open Finance Webhook endpoints (exempt from TenantContextMiddleware)
	if openFinanceHandler != nil {
		r.Post("/open-finance/webhook", openFinanceHandler.HandleWebhook)
		r.Post("/api/v1/open-finance/webhook", openFinanceHandler.HandleWebhook)
	}

	// Route registration helper for protected endpoints
	registerAPIRoutes := func(sub chi.Router) {
		if cfg.AppConfig != nil && cfg.AppConfig.Environment == "personal" {
			// Global authentication already supplied the only trusted identity.
		} else {
			if cfg.AppConfig != nil {
				sub.Use(middleware.DevAdminFallbackMiddleware(
					cfg.AppConfig.IsDevelopment(),
					cfg.AppConfig.DevTenantID,
					cfg.AppConfig.DevUserID,
				))
			}
			sub.Use(middleware.TenantContextMiddleware)
		}

		if sessionHandler != nil {
			sub.Get("/session", sessionHandler.GetSession)
		}

		// Accounts CRUD
		sub.Route("/accounts", func(r chi.Router) {
			r.Post("/", accountHandler.CreateAccount)
			r.Get("/", accountHandler.ListAccounts)
			r.Get("/{id}", accountHandler.GetAccount)
			r.Patch("/{id}", accountHandler.UpdateAccount)
		})

		// Categories CRUD
		sub.Route("/categories", func(r chi.Router) {
			r.Get("/", categoryHandler.ListCategories)
			r.Post("/", categoryHandler.CreateCategory)
			r.Patch("/{id}", categoryHandler.UpdateCategory)
		})

		// Transactions CRUD
		sub.Route("/transactions", func(r chi.Router) {
			r.Post("/", transactionHandler.CreateTransaction)
			r.Get("/", transactionHandler.ListTransactions)
			r.Get("/summary", transactionHandler.SummarizeTransactions)
			r.Post("/batch-delete", transactionHandler.BatchDeleteTransactions)
			r.Post("/batch-category", transactionHandler.BatchUpdateCategory)
			if csvHandler != nil {
				r.Post("/import/csv", csvHandler.ImportCSV)
			}
			r.Get("/{id}", transactionHandler.GetTransaction)
			r.Put("/{id}", transactionHandler.UpdateTransaction)
			r.Patch("/{id}", transactionHandler.UpdateTransaction)
			r.Delete("/{id}", transactionHandler.DeleteTransaction)
		})

		// Transfers CRUD & Pairing
		if transferHandler != nil {
			sub.Route("/transfers", func(r chi.Router) {
				r.Post("/confirm", transferHandler.ConfirmTransfer)
				r.Post("/", transferHandler.CreateTransfer)
				r.Post("/unlink", transferHandler.UnlinkTransfer)
			})
		}

		if pendingActionHandler != nil {
			sub.Route("/pending-actions", func(r chi.Router) {
				r.Get("/", pendingActionHandler.ListPending)
				r.Post("/{id}/approve", pendingActionHandler.Approve)
				r.Post("/{id}/reject", pendingActionHandler.Reject)
			})
		}
	}

	// 3. Register under /api/v1
	r.Route("/api/v1", registerAPIRoutes)

	// 4. Also register under root Group for direct endpoint access
	r.Group(registerAPIRoutes)

	// 5. Embedded Frontend Single Page Application (SPA)
	RegisterSPARoutes(r)

	return r
}
