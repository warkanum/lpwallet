# lpwallet Implementation Progress

## Phase 1: Project Foundation
- [x] todo.md created
- [x] go.mod — dependencies added (gorm, postgres, yaml.v3, testify)
- [x] Makefile

## Phase 2: Config
- [x] internal/config/config.go — struct, Load(), DSN(), YAML+env merge, validation

## Phase 3: Database
- [x] internal/db/db.go — Open(), GORM connection
- [x] internal/db/audit_plugin.go — GORM plugin for audited tables (user, account, account_transaction)
- [x] DBML updated: id_user, id_account, id_audit_event changed to bigserial/autoIncrement
- [x] Models regenerated via relspec

## Phase 4: Middleware
- [x] internal/middleware/auth.go — context key, WithUser, UserFromContext, Auth middleware (with skip paths)
- [x] internal/middleware/logging.go — request logging (method, path, status, duration_ms, request_id)
- [x] internal/middleware/recovery.go — panic recovery
- [x] internal/requestid/requestid.go — request ID generation and propagation

## Phase 5: Handlers
- [x] internal/handlers/helpers.go — Handler struct, writeJSON, writeError, parseJSON
- [x] internal/handlers/public.go — GET /healthz, GET /version, GET /, GET /api/v1/openapi.json, GET /api/v1/swagger/
- [x] internal/handlers/auth.go — POST /api/v1/auth/login, POST /api/v1/auth/logout
- [x] internal/handlers/users.go — CRUD /api/v1/users
- [x] internal/handlers/accounts.go — CRUD /api/v1/accounts
- [x] internal/handlers/transactions.go — CRUD + batch /api/v1/transactions (with balance recalc)

## Phase 6: Server
- [x] cmd/server/main.go — router, middleware stack (recovery→logging→requestID→auth), http.Server, TLS, admin seed

## Phase 7: Entrypoint
- [x] main.go — delegates to cmd/server.Run()

## Phase 8: OpenAPI
- [x] openapi.json — OpenAPI 3.1 spec (all endpoints)
- [x] Swagger UI CDN handler (unpkg.com/swagger-ui-dist)

## Phase 9: Tests
- [x] internal/config/config_test.go — defaults, DSN, validation, env override
- [x] internal/middleware/auth_test.go — context round-trip, bearer parsing
- [x] internal/handlers/helpers_test.go — writeJSON, writeError, isAdmin, isUniqueViolation

## Remaining / Future
- [ ] Integration tests (require a real PostgreSQL DB)
- [ ] Handler tests for users, accounts, transactions (need DB mock or real DB)
- [ ] DDL migration script (CREATE TABLE statements matching DBML)

