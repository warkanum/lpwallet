package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

type accountInput struct {
	RIDUser int64   `json:"rid_user"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
}

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())

	var accounts []models.ModelPublicAccount
	q := h.db.WithContext(r.Context())
	if !isAdmin(caller) {
		q = q.Where("rid_user = ?", caller.IDUser)
	}
	if err := q.Find(&accounts).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}

	caller, _ := middleware.UserFromContext(r.Context())

	var account models.ModelPublicAccount
	if err := h.db.WithContext(r.Context()).First(&account, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "account not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	if !isAdmin(caller) && account.RIDUser.Int64 != caller.IDUser {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	var inp accountInput
	if err := parseJSON(r, &inp); err != nil || inp.Name == "" || inp.RIDUser == 0 {
		writeError(w, http.StatusBadRequest, "name and rid_user required", "bad_request")
		return
	}

	account := models.ModelPublicAccount{
		RIDUser: sql.NullInt64{Int64: inp.RIDUser, Valid: true},
		Name:    sql.NullString{String: inp.Name, Valid: true},
		Balance: sql.NullFloat64{Float64: inp.Balance, Valid: true},
	}

	err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&account).Error
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "account name already exists for this user", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
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

	var inp accountInput
	if err := parseJSON(r, &inp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	var account models.ModelPublicAccount
	err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&account, id).Error; err != nil {
			return err
		}
		if inp.Name != "" {
			account.Name = sql.NullString{String: inp.Name, Valid: true}
		}
		return tx.Save(&account).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "account not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
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
		return tx.Delete(&models.ModelPublicAccount{}, id).Error
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
