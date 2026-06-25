package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/config"
	lpdb "github.com/warkanum/lpwallet/internal/db"
	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

func newTxTestHandler(t *testing.T) *Handler {
	t.Helper()
	f, err := os.CreateTemp("", "lpwallet-test-*.db")
	require.NoError(t, err)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	cfg := &config.Config{DBDriver: "sqlite", DBFile: f.Name()}
	db, err := lpdb.Open(cfg)
	require.NoError(t, err)
	return New(db, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
}

func seedTxUser(t *testing.T, db *gorm.DB) *models.ModelPublicUser {
	t.Helper()
	u := &models.ModelPublicUser{
		Email: sql.NullString{String: fmt.Sprintf("u%d@test.local", time.Now().UnixNano()), Valid: true},
		Role:  sql.NullString{String: "admin", Valid: true},
		Name:  sql.NullString{String: "Test User", Valid: true},
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

func seedTxAccount(t *testing.T, db *gorm.DB, userID int64) *models.ModelPublicAccount {
	t.Helper()
	a := &models.ModelPublicAccount{
		RIDUser: sql.NullInt64{Int64: userID, Valid: true},
		Name:    sql.NullString{String: fmt.Sprintf("acct%d", time.Now().UnixNano()), Valid: true},
		Balance: sql.NullFloat64{Float64: 0, Valid: true},
	}
	require.NoError(t, db.Create(a).Error)
	return a
}

// txResponse mirrors the JSON shape produced by ModelPublicAccountTransaction.MarshalJSON.
type txResponse struct {
	IDAccountTransaction int64    `json:"id_account_transaction"`
	Action               *string  `json:"action"`
	Amount               *float64 `json:"amount"`
	Reference            *string  `json:"reference"`
	RIDAccount           *int64   `json:"rid_account"`
}

func postTransaction(h *Handler, user *models.ModelPublicUser, inp transactionInput) (int, *txResponse, *errorResponse) {
	body, _ := json.Marshal(inp)
	r := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewReader(body))
	r = r.WithContext(middleware.WithUser(r.Context(), user))
	w := httptest.NewRecorder()
	h.CreateTransaction(w, r)

	if w.Code == http.StatusCreated {
		var tx txResponse
		if err := json.Unmarshal(w.Body.Bytes(), &tx); err == nil {
			return w.Code, &tx, nil
		}
	}
	var errResp errorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp) //nolint:errcheck
	return w.Code, nil, &errResp
}

func getBalance(t *testing.T, db *gorm.DB, accountID int64) float64 {
	t.Helper()
	var a models.ModelPublicAccount
	require.NoError(t, db.First(&a, accountID).Error)
	return a.Balance.Float64
}

func TestEarnTransaction(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	code, tx, _ := postTransaction(h, user, transactionInput{
		RIDAccount: account.IDAccount,
		Action:     "earn",
		Reference:  "REF-EARN-001",
		Amount:     100.0,
	})

	require.Equal(t, http.StatusCreated, code)
	require.NotNil(t, tx)
	assert.Equal(t, "earn", *tx.Action)
	assert.Equal(t, 100.0, *tx.Amount)
	assert.Equal(t, 100.0, getBalance(t, h.db, account.IDAccount))
}

func TestEarnThenSpendTransaction(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	code, _, _ := postTransaction(h, user, transactionInput{
		RIDAccount: account.IDAccount,
		Action:     "earn",
		Reference:  "REF-EARN-001",
		Amount:     100.0,
	})
	require.Equal(t, http.StatusCreated, code)

	code, _, _ = postTransaction(h, user, transactionInput{
		RIDAccount: account.IDAccount,
		Action:     "spend",
		Reference:  "REF-SPEND-001",
		Amount:     30.0,
	})
	require.Equal(t, http.StatusCreated, code)

	assert.Equal(t, 70.0, getBalance(t, h.db, account.IDAccount))
}

func TestSpendInsufficientBalance(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	code, _, _ := postTransaction(h, user, transactionInput{
		RIDAccount: account.IDAccount,
		Action:     "earn",
		Reference:  "REF-EARN-001",
		Amount:     50.0,
	})
	require.Equal(t, http.StatusCreated, code)

	code, _, errResp := postTransaction(h, user, transactionInput{
		RIDAccount: account.IDAccount,
		Action:     "spend",
		Reference:  "REF-SPEND-001",
		Amount:     100.0,
	})

	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "insufficient_balance", errResp.Code)
	assert.Equal(t, 50.0, getBalance(t, h.db, account.IDAccount))
}

func TestDuplicateTransactionReference(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	inp := transactionInput{
		RIDAccount: account.IDAccount,
		Action:     "earn",
		Reference:  "REF-DUP",
		Amount:     50.0,
	}

	code, _, _ := postTransaction(h, user, inp)
	require.Equal(t, http.StatusCreated, code)

	code, _, errResp := postTransaction(h, user, inp)
	assert.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "conflict", errResp.Code)
	assert.Equal(t, 50.0, getBalance(t, h.db, account.IDAccount))
}

func TestConcurrentEarnTransactions(t *testing.T) {
	h := newTxTestHandler(t)

	// Serialize SQLite writes to prevent "database is locked" under concurrent load.
	sqlDB, err := h.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	const workers = 10
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		txErrs  []string
	)
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			code, _, errResp := postTransaction(h, user, transactionInput{
				RIDAccount: account.IDAccount,
				Action:     "earn",
				Reference:  fmt.Sprintf("REF-CONCURRENT-%02d", i),
				Amount:     10.0,
			})
			if code != http.StatusCreated {
				mu.Lock()
				txErrs = append(txErrs, fmt.Sprintf("worker %d: %s", i, errResp.Error))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Empty(t, txErrs, "unexpected errors during concurrent earns: %v", txErrs)
	assert.Equal(t, float64(workers)*10.0, getBalance(t, h.db, account.IDAccount))
}

// postCSV posts a CSV body to the batch CSV endpoint and returns status + raw body.
func postCSV(h *Handler, user *models.ModelPublicUser, csvBody string) (int, []byte) {
	r := httptest.NewRequest(http.MethodPost, "/transactions/batch/csv", bytes.NewBufferString(csvBody))
	r.Header.Set("Content-Type", "text/csv")
	r = r.WithContext(middleware.WithUser(r.Context(), user))
	w := httptest.NewRecorder()
	h.CreateTransactionBatchCSV(w, r)
	return w.Code, w.Body.Bytes()
}

func TestCSVImport_EarnAndSpend(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	csv := fmt.Sprintf("ref,account_id,kind,points,occurred_at\n"+
		"CSV-EARN-001,%d,earn,200,2024-01-01T10:00:00Z\n"+
		"CSV-SPEND-001,%d,spend,50,2024-01-02T10:00:00Z\n",
		account.IDAccount, account.IDAccount)

	code, body := postCSV(h, user, csv)

	require.Equal(t, http.StatusCreated, code, "body: %s", body)

	var results []txResponse
	require.NoError(t, json.Unmarshal(body, &results))
	assert.Len(t, results, 2)
	assert.Equal(t, "earn", *results[0].Action)
	assert.Equal(t, "spend", *results[1].Action)
	assert.Equal(t, 150.0, getBalance(t, h.db, account.IDAccount))
}

func TestCSVImport_MissingHeader(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)

	code, body := postCSV(h, user, "ref,account_id,kind\nCSV-001,1,earn\n")

	assert.Equal(t, http.StatusBadRequest, code)
	var errResp errorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "bad_request", errResp.Code)
	assert.Contains(t, errResp.Error, "points")
}

func TestCSVImport_UnknownAccount(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)

	code, body := postCSV(h, user, "ref,account_id,kind,points,occurred_at\nCSV-001,99999,earn,100,\n")

	assert.Equal(t, http.StatusNotFound, code)
	var errResp errorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "not_found", errResp.Code)
}

func TestCSVImport_DuplicateReferenceRollback(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	// Pre-seed REF-DUP so the CSV row conflicts.
	code, _, _ := postTransaction(h, user, transactionInput{
		RIDAccount: account.IDAccount, Action: "earn", Reference: "REF-DUP", Amount: 100.0,
	})
	require.Equal(t, http.StatusCreated, code)

	csv := fmt.Sprintf("ref,account_id,kind,points,occurred_at\n"+
		"CSV-NEW,%d,earn,50,\n"+
		"REF-DUP,%d,earn,50,\n",
		account.IDAccount, account.IDAccount)

	code, body := postCSV(h, user, csv)

	assert.Equal(t, http.StatusConflict, code, "body: %s", body)
	// Whole batch rolled back — balance unchanged from the pre-seeded earn.
	assert.Equal(t, 100.0, getBalance(t, h.db, account.IDAccount))
}

func TestCSVImport_SpendInsufficientBalance(t *testing.T) {
	h := newTxTestHandler(t)
	user := seedTxUser(t, h.db)
	account := seedTxAccount(t, h.db, user.IDUser)

	csv := fmt.Sprintf("ref,account_id,kind,points,occurred_at\n"+
		"CSV-EARN,%d,earn,30,\n"+
		"CSV-SPEND,%d,spend,100,\n",
		account.IDAccount, account.IDAccount)

	code, body := postCSV(h, user, csv)

	assert.Equal(t, http.StatusBadRequest, code, "body: %s", body)
	var errResp errorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "insufficient_balance", errResp.Code)
	// Entire batch rolled back — balance remains 0.
	assert.Equal(t, 0.0, getBalance(t, h.db, account.IDAccount))
}
