package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

type userInput struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Password     string `json:"password"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}


func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	var users []models.ModelPublicUser
	if err := h.db.WithContext(r.Context()).Find(&users).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}

	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) && caller.IDUser != id {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	var user models.ModelPublicUser
	if err := h.db.WithContext(r.Context()).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "user not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	var inp userInput
	if err := parseJSON(r, &inp); err != nil || inp.Email == "" || inp.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required", "bad_request")
		return
	}

	role := inp.Role
	if role != "admin" && role != "member" {
		role = "member"
	}

	user := models.ModelPublicUser{
		Name:         sql.NullString{String: inp.Name, Valid: inp.Name != ""},
		Email:        sql.NullString{String: inp.Email, Valid: true},
		Role:         sql.NullString{String: role, Valid: true},
		Password:     sql.NullString{String: hashPassword(inp.Password), Valid: true},
		ClientID:     sql.NullString{String: inp.ClientID, Valid: inp.ClientID != ""},
		ClientSecret: sql.NullString{String: inp.ClientSecret, Valid: inp.ClientSecret != ""},
	}

	err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&user).Error
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "email already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}

	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) && caller.IDUser != id {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	var inp userInput
	if err := parseJSON(r, &inp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	var user models.ModelPublicUser
	err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		if inp.Name != "" {
			user.Name = sql.NullString{String: inp.Name, Valid: true}
		}
		if inp.Email != "" {
			user.Email = sql.NullString{String: inp.Email, Valid: true}
		}
		if inp.Password != "" {
			user.Password = sql.NullString{String: hashPassword(inp.Password), Valid: true}
		}
		if inp.Role != "" && isAdmin(caller) {
			user.Role = sql.NullString{String: inp.Role, Valid: true}
		}
		if inp.ClientID != "" {
			user.ClientID = sql.NullString{String: inp.ClientID, Valid: true}
		}
		if inp.ClientSecret != "" {
			user.ClientSecret = sql.NullString{String: inp.ClientSecret, Valid: true}
		}
		return tx.Save(&user).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "user not found", "not_found")
			return
		}
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "email already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
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
		return tx.Delete(&models.ModelPublicUser{}, id).Error
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isAdmin(u *models.ModelPublicUser) bool {
	return u != nil && u.Role.Valid && u.Role.String == "admin"
}

func pathID(r *http.Request, key string) (int64, bool) {
	s := r.PathValue(key)
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(err.Error(), "duplicate key", "unique constraint", "23505")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
