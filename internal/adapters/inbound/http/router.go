package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/handlers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

// RouterConfig contains dependencies and configuration options for assembling the API router.
type RouterConfig struct {
	AccountService     ports.AccountService
	CategoryService    ports.CategoryService
	TransactionService ports.TransactionService
	Logger             *slog.Logger
	CORSOptions        *cors.Options
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

	// Route registration helper for protected endpoints
	registerAPIRoutes := func(sub chi.Router) {
		sub.Use(middleware.TenantContextMiddleware)

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
			r.Get("/{id}", transactionHandler.GetTransaction)
			r.Patch("/{id}", transactionHandler.UpdateTransaction)
			r.Delete("/{id}", transactionHandler.DeleteTransaction)
		})
	}

	// 3. Register under /api/v1
	r.Route("/api/v1", registerAPIRoutes)

	// 4. Also register under root Group for direct endpoint access
	r.Group(registerAPIRoutes)

	return r
}
