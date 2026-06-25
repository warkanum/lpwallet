package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

const sessionTTL = 24 * time.Hour

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AuthToken string `json:"authtoken"`
	ExpiresAt string `json:"expires_at"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := parseJSON(r, &req); err != nil || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password required", "bad_request")
		return
	}

	hashed := hashPassword(req.Password)

	var user models.ModelPublicUser
	err := h.db.WithContext(r.Context()).
		Where("email = ? AND password = ?", req.Email, hashed).
		First(&user).Error
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials", "unauthorized")
		return
	}

	token := generateToken()
	expiresAt := time.Now().Add(sessionTTL)

	session := models.ModelPublicUserSession{
		RIDUser:   sql.NullInt64{Int64: user.IDUser, Valid: true},
		Createdat: sql.NullTime{Time: time.Now(), Valid: true},
		Expiresat: sql.NullTime{Time: expiresAt, Valid: true},
		Authtoken: sql.NullString{String: token, Valid: true},
	}

	if err := h.db.WithContext(r.Context()).Create(&session).Error; err != nil {
		h.logger.Error("login: create session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		AuthToken: token,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	err := h.db.WithContext(r.Context()).
		Model(&models.ModelPublicUserSession{}).
		Where("rid_user = ? AND expiresat > ?", user.IDUser, time.Now()).
		Update("expiresat", time.Now()).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}
