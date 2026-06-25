package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	lpdb "github.com/warkanum/lpwallet/internal/db"
	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

var csvOccurredAtFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

type transactionInput struct {
	RIDAccount  int64   `json:"rid_account"`
	Action      string  `json:"action"`
	Reference   string  `json:"reference"`
	Amount      float64 `json:"amount"`
	TransactedAt string  `json:"transacted_at"`
}

type batchRequest struct {
	Transactions []transactionInput `json:"transactions"`
}

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())

	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr == "" {
		writeError(w, http.StatusBadRequest, "account_id query param required", "bad_request")
		return
	}

	accountID, ok2 := parseIntStr(accountIDStr)
	if !ok2 {
		writeError(w, http.StatusBadRequest, "invalid account_id", "bad_request")
		return
	}

	var account models.ModelPublicAccount
	if err := h.db.WithContext(r.Context()).First(&account, accountID).Error; err != nil {
		writeError(w, http.StatusNotFound, "account not found", "not_found")
		return
	}
	if !isAdmin(caller) && account.RIDUser.Int64 != caller.IDUser {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	var txs []models.ModelPublicAccountTransaction
	if err := h.db.WithContext(r.Context()).Where("rid_account = ?", accountID).Find(&txs).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, txs)
}

func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}

	caller, _ := middleware.UserFromContext(r.Context())

	var tx models.ModelPublicAccountTransaction
	if err := h.db.WithContext(r.Context()).First(&tx, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	var account models.ModelPublicAccount
	if err := h.db.WithContext(r.Context()).First(&account, tx.RIDAccount.Int64).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	if !isAdmin(caller) && account.RIDUser.Int64 != caller.IDUser {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, tx)
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var inp transactionInput
	if err := parseJSON(r, &inp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	caller, _ := middleware.UserFromContext(r.Context())

	created, httpStatus, errMsg, errCode := h.captureTransaction(r, caller, inp)
	if errMsg != "" {
		writeError(w, httpStatus, errMsg, errCode)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) CreateTransactionBatch(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := parseJSON(r, &req); err != nil || len(req.Transactions) == 0 {
		writeError(w, http.StatusBadRequest, "transactions array required", "bad_request")
		return
	}

	caller, _ := middleware.UserFromContext(r.Context())
	results := make([]models.ModelPublicAccountTransaction, 0, len(req.Transactions))

	for _, inp := range req.Transactions {
		created, httpStatus, errMsg, errCode := h.captureTransaction(r, caller, inp)
		if errMsg != "" {
			writeError(w, httpStatus, errMsg, errCode)
			return
		}
		results = append(results, *created)
	}

	writeJSON(w, http.StatusCreated, results)
}

func (h *Handler) captureTransaction(
	r *http.Request,
	caller *models.ModelPublicUser,
	inp transactionInput,
) (*models.ModelPublicAccountTransaction, int, string, string) {
	if inp.RIDAccount == 0 || inp.Reference == "" || inp.Action == "" {
		return nil, http.StatusBadRequest, "rid_account, reference and action required", "bad_request"
	}
	if inp.Action != "spend" && inp.Action != "earn" {
		return nil, http.StatusBadRequest, "action must be 'spend' or 'earn'", "bad_request"
	}

	var account models.ModelPublicAccount
	if err := h.db.WithContext(r.Context()).First(&account, inp.RIDAccount).Error; err != nil {
		return nil, http.StatusNotFound, "account not found", "not_found"
	}
	if !isAdmin(caller) && account.RIDUser.Int64 != caller.IDUser {
		return nil, http.StatusForbidden, "forbidden", "forbidden"
	}

	record := buildTransactionRecord(inp)

	err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return recalcBalance(tx, inp.RIDAccount)
	})
	if err != nil {
		status, msg, code := mapTxError(err)
		return nil, status, msg, code
	}
	return &record, http.StatusCreated, "", ""
}

func buildTransactionRecord(inp transactionInput) models.ModelPublicAccountTransaction {
	txDatetime := time.Now()
	if inp.TransactedAt != "" {
		if t, err := time.Parse(time.RFC3339, inp.TransactedAt); err == nil {
			txDatetime = t
		}
	}
	return models.ModelPublicAccountTransaction{
		RIDAccount:          sql.NullInt64{Int64: inp.RIDAccount, Valid: true},
		Action:              sql.NullString{String: inp.Action, Valid: true},
		Reference:           sql.NullString{String: inp.Reference, Valid: true},
		Amount:              sql.NullFloat64{Float64: inp.Amount, Valid: true},
		TransactionDatetime: sql.NullTime{Time: txDatetime, Valid: true},
	}
}

func mapTxError(err error) (int, string, string) {
	switch {
	case errors.Is(err, lpdb.ErrDuplicateTransaction):
		return http.StatusConflict, "transaction reference already captured for this account", "conflict"
	case errors.Is(err, lpdb.ErrInsufficientBalance):
		return http.StatusBadRequest, "insufficient balance for spend", "insufficient_balance"
	case isUniqueViolation(err):
		return http.StatusConflict, "duplicate reference for this account", "conflict"
	default:
		return http.StatusInternalServerError, "internal server error", "internal_error"
	}
}

func (h *Handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}

	err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		var record models.ModelPublicAccountTransaction
		if err := tx.First(&record, id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&record).Error; err != nil {
			return err
		}
		return recalcBalance(tx, record.RIDAccount.Int64)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func recalcBalance(tx *gorm.DB, accountID int64) error {
	type result struct {
		Balance float64
	}
	var res result
	err := tx.Model(&models.ModelPublicAccountTransaction{}).
		Select("COALESCE(SUM(CASE WHEN action='earn' THEN amount ELSE -amount END), 0) AS balance").
		Where("rid_account = ?", accountID).
		Scan(&res).Error
	if err != nil {
		return err
	}
	return tx.Model(&models.ModelPublicAccount{}).
		Where("id_account = ?", accountID).
		Update("balance", res.Balance).Error
}

func parseIntStr(s string) (int64, bool) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	return n, s != ""
}

// CreateTransactionBatchCSV accepts a CSV body (Content-Type: text/csv).
// Header row must be: ref,account_id,kind,points,occurred_at
// account_id is the integer id_account.
func (h *Handler) CreateTransactionBatchCSV(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())

	rdr := csv.NewReader(r.Body)
	rdr.TrimLeadingSpace = true

	header, err := rdr.Read()
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read CSV header", "bad_request")
		return
	}
	colIdx, err := csvColumnIndex(header)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	type parsedRow struct {
		lineNum int
		inp     transactionInput
		record  models.ModelPublicAccountTransaction
	}

	// Pass 1: parse, field-validate, and ownership-check every row before touching the DB.
	var rows []parsedRow
	lineNum := 1
	for {
		row, err := rdr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		lineNum++
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("row %d: malformed CSV: %v", lineNum, err), "bad_request")
			return
		}
		inp, err := csvRowToInput(row, colIdx)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("row %d: %v", lineNum, err), "bad_request")
			return
		}
		if inp.RIDAccount == 0 || inp.Reference == "" || inp.Action == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("row %d: rid_account, reference and action required", lineNum), "bad_request")
			return
		}
		if inp.Action != "spend" && inp.Action != "earn" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("row %d: action must be 'spend' or 'earn'", lineNum), "bad_request")
			return
		}
		var account models.ModelPublicAccount
		if err := h.db.WithContext(r.Context()).First(&account, inp.RIDAccount).Error; err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("row %d: account %d not found", lineNum, inp.RIDAccount), "not_found")
			return
		}
		if !isAdmin(caller) && account.RIDUser.Int64 != caller.IDUser {
			writeError(w, http.StatusForbidden, fmt.Sprintf("row %d: forbidden", lineNum), "forbidden")
			return
		}
		rows = append(rows, parsedRow{lineNum: lineNum, inp: inp, record: buildTransactionRecord(inp)})
	}

	if len(rows) == 0 {
		writeError(w, http.StatusBadRequest, "CSV contained no data rows", "bad_request")
		return
	}

	// Pass 2: all creates + balance recalcs in a single atomic transaction.
	var failedLine int
	var failedErr error

	dbErr := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		for i := range rows {
			if err := tx.Create(&rows[i].record).Error; err != nil {
				failedLine, failedErr = rows[i].lineNum, err
				return err
			}
			if err := recalcBalance(tx, rows[i].inp.RIDAccount); err != nil {
				failedLine, failedErr = rows[i].lineNum, err
				return err
			}
		}
		return nil
	})

	if dbErr != nil {
		status, msg, code := mapTxError(failedErr)
		writeError(w, status, fmt.Sprintf("row %d: %s", failedLine, msg), code)
		return
	}

	results := make([]models.ModelPublicAccountTransaction, len(rows))
	for i := range rows {
		results[i] = rows[i].record
	}
	writeJSON(w, http.StatusCreated, results)
}

var requiredCSVColumns = []string{"ref", "account_id", "kind", "points", "occurred_at"}

func csvColumnIndex(header []string) (map[string]int, error) {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[col] = i
	}
	for _, required := range requiredCSVColumns {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("missing required column %q", required)
		}
	}
	return idx, nil
}

func csvRowToInput(row []string, colIdx map[string]int) (transactionInput, error) {
	get := func(col string) string {
		i, ok := colIdx[col]
		if !ok || i >= len(row) {
			return ""
		}
		return row[i]
	}

	accountID, err := strconv.ParseInt(get("account_id"), 10, 64)
	if err != nil {
		return transactionInput{}, fmt.Errorf("account_id %q is not a valid integer", get("account_id"))
	}

	points, err := strconv.ParseFloat(get("points"), 64)
	if err != nil {
		return transactionInput{}, fmt.Errorf("points %q is not a valid number", get("points"))
	}

	occurredAt, err := parseOccurredAt(get("occurred_at"))
	if err != nil {
		return transactionInput{}, fmt.Errorf("occurred_at %q: %v", get("occurred_at"), err)
	}

	return transactionInput{
		RIDAccount:   accountID,
		Action:       get("kind"),
		Reference:    get("ref"),
		Amount:       points,
		TransactedAt: occurredAt.UTC().Format(time.RFC3339),
	}, nil
}

func parseOccurredAt(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	for _, layout := range csvOccurredAtFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised datetime format")
}
