package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// RouterConfig contains dependencies and configuration options for assembling the API router.
type RouterConfig struct {
	AccountService      ports.AccountService
	CategoryService     ports.CategoryService
	TransactionService  ports.TransactionService
	CSVIngestionService ports.CSVIngestionService
	AppConfig           *config.Config
	Logger              *slog.Logger
	CORSOptions         *cors.Options
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"READY"}`))
	})

	// Handlers initialization
	accountHandler := handlers.NewAccountHandler(cfg.AccountService)
	categoryHandler := handlers.NewCategoryHandler(cfg.CategoryService)
	transactionHandler := handlers.NewTransactionHandler(cfg.TransactionService)
	var csvHandler *handlers.CSVIngestHandler
	if cfg.CSVIngestionService != nil {
		csvHandler = handlers.NewCSVIngestHandler(cfg.CSVIngestionService)
	}
	var sessionHandler *handlers.SessionHandler
	if cfg.AppConfig != nil {
		sessionHandler = handlers.NewSessionHandler(cfg.AppConfig)
	}

	// Route registration helper for protected endpoints
	registerAPIRoutes := func(sub chi.Router) {
		if cfg.AppConfig != nil {
			sub.Use(middleware.DevAdminFallbackMiddleware(
				cfg.AppConfig.IsDevelopment(),
				cfg.AppConfig.DevTenantID,
				cfg.AppConfig.DevUserID,
			))
		}
		sub.Use(middleware.TenantContextMiddleware)

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
	}

	// 3. Register under /api/v1
	r.Route("/api/v1", registerAPIRoutes)

	// 4. Also register under root Group for direct endpoint access
	r.Group(registerAPIRoutes)

	// 5. Embedded Frontend Single Page Application (SPA)
	RegisterSPARoutes(r)

	return r
}
