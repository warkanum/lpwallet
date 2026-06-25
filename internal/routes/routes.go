package routes

import (
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/handlers"
	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/requestid"
)

// New wires all routes and middleware and returns the root http.Handler.
func New(db *gorm.DB, logger *slog.Logger) http.Handler {
	h := handlers.New(db, logger)

	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /version", h.VersionHandler)
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /api/v1/openapi.json", h.OpenAPI)
	mux.HandleFunc("GET /api/v1/swagger/", h.SwaggerUI)

	// Auth
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)

	// Users
	mux.HandleFunc("GET /api/v1/users", h.ListUsers)
	mux.HandleFunc("GET /api/v1/users/{id}", h.GetUser)
	mux.HandleFunc("POST /api/v1/users", h.CreateUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", h.UpdateUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", h.DeleteUser)

	// Accounts
	mux.HandleFunc("GET /api/v1/accounts", h.ListAccounts)
	mux.HandleFunc("GET /api/v1/accounts/{id}", h.GetAccount)
	mux.HandleFunc("POST /api/v1/accounts", h.CreateAccount)
	mux.HandleFunc("PUT /api/v1/accounts/{id}", h.UpdateAccount)
	mux.HandleFunc("DELETE /api/v1/accounts/{id}", h.DeleteAccount)

	// Audit (admin only)
	mux.HandleFunc("GET /api/v1/audit", h.ListAuditEvents)
	mux.HandleFunc("GET /api/v1/audit/report", h.GetAuditReport)

	// Transactions
	mux.HandleFunc("GET /api/v1/transactions", h.ListTransactions)
	mux.HandleFunc("GET /api/v1/transactions/{id}", h.GetTransaction)
	mux.HandleFunc("POST /api/v1/transactions", h.CreateTransaction)
	mux.HandleFunc("POST /api/v1/transactions/batch", h.CreateTransactionBatch)
	mux.HandleFunc("POST /api/v1/transactions/batch/csv", h.CreateTransactionBatchCSV)
	mux.HandleFunc("DELETE /api/v1/transactions/{id}", h.DeleteTransaction)

	publicPaths := []string{
		"/healthz", "/version", "/",
		"/api/v1/openapi.json", "/api/v1/swagger/", "/api/v1/swagger",
		"/api/v1/auth/login",
	}

	var handler http.Handler = mux
	handler = middleware.Auth(db, publicPaths...)(handler)
	handler = middleware.Logging(logger)(handler)
	handler = requestid.WithRequestID(handler)
	handler = middleware.Recovery(logger)(handler)

	return handler
}
