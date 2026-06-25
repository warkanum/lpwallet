package handlers

import (
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

var auditDateFormats = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}

func parseAuditDate(s string) (time.Time, bool) {
	for _, layout := range auditDateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

type auditFilters struct {
	from   *time.Time
	to     *time.Time
	table  string
	userID int64
}

func parseAuditFilters(r *http.Request) (auditFilters, string) {
	var f auditFilters
	if s := r.URL.Query().Get("from"); s != "" {
		t, ok := parseAuditDate(s)
		if !ok {
			return f, "invalid from date"
		}
		f.from = &t
	}
	if s := r.URL.Query().Get("to"); s != "" {
		t, ok := parseAuditDate(s)
		if !ok {
			return f, "invalid to date"
		}
		f.to = &t
	}
	if s := r.URL.Query().Get("table"); s != "" {
		f.table = s
	}
	if s := r.URL.Query().Get("user_id"); s != "" {
		uid, ok := parseIntStr(s)
		if !ok {
			return f, "invalid user_id"
		}
		f.userID = uid
	}
	return f, ""
}

func applyAuditFilters(q *gorm.DB, f auditFilters) *gorm.DB {
	if f.from != nil {
		q = q.Where("datetime >= ?", *f.from)
	}
	if f.to != nil {
		q = q.Where("datetime <= ?", *f.to)
	}
	if f.table != "" {
		q = q.Where("tablename = ?", f.table)
	}
	if f.userID != 0 {
		q = q.Where("rid_user = ?", f.userID)
	}
	return q
}

// ListAuditEvents handles GET /api/v1/audit
// Admin only. Query params: from, to, table, user_id
func (h *Handler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	f, errMsg := parseAuditFilters(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg, "bad_request")
		return
	}

	q := applyAuditFilters(
		h.db.WithContext(r.Context()).Preload("RelRIDAuditDetailPublicAuditDetails").Order("datetime DESC"),
		f,
	)

	var events []models.ModelPublicAuditEvent
	if err := q.Find(&events).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

type auditReportTableSummary struct {
	Table   string `json:"table"`
	Creates int64  `json:"creates"`
	Updates int64  `json:"updates"`
	Deletes int64  `json:"deletes"`
	Total   int64  `json:"total"`
}

type auditReportUserSummary struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email,omitempty"`
	Total  int64  `json:"total"`
}

type auditReportDaySummary struct {
	Date  string `json:"date"`
	Total int64  `json:"total"`
}

type auditReport struct {
	FromDate    *string                   `json:"from_date,omitempty"`
	ToDate      *string                   `json:"to_date,omitempty"`
	TotalEvents int64                     `json:"total_events"`
	ByTable     []auditReportTableSummary `json:"by_table"`
	ByUser      []auditReportUserSummary  `json:"by_user"`
	ByDay       []auditReportDaySummary   `json:"by_day"`
}

// GetAuditReport handles GET /api/v1/audit/report
// Admin only. Query params: from, to
func (h *Handler) GetAuditReport(w http.ResponseWriter, r *http.Request) {
	caller, _ := middleware.UserFromContext(r.Context())
	if !isAdmin(caller) {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	f, errMsg := parseAuditFilters(r)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg, "bad_request")
		return
	}

	db := h.db.WithContext(r.Context())

	// Total
	var total int64
	if err := applyAuditFilters(db.Model(&models.ModelPublicAuditEvent{}), f).Count(&total).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}

	// By table + action
	type tableRow struct {
		Tablename string
		Action    int16
		Cnt       int64
	}
	var tableRows []tableRow
	if err := applyAuditFilters(db.Model(&models.ModelPublicAuditEvent{}), f).
		Select("tablename, action, COUNT(*) AS cnt").
		Group("tablename, action").
		Order("tablename, action").
		Scan(&tableRows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	byTableMap := map[string]*auditReportTableSummary{}
	for _, row := range tableRows {
		s := byTableMap[row.Tablename]
		if s == nil {
			s = &auditReportTableSummary{Table: row.Tablename}
			byTableMap[row.Tablename] = s
		}
		switch row.Action {
		case 1:
			s.Creates += row.Cnt
		case 2:
			s.Updates += row.Cnt
		case 3:
			s.Deletes += row.Cnt
		}
		s.Total += row.Cnt
	}
	byTable := make([]auditReportTableSummary, 0, len(byTableMap))
	for _, v := range byTableMap {
		byTable = append(byTable, *v)
	}

	// By user with email join
	type userRow struct {
		RIDUser int64
		Email   string
		Cnt     int64
	}
	var userRows []userRow
	userQ := applyAuditFilters(
		db.Table("public.audit_event ae").
			Select("ae.rid_user, COALESCE(u.email, '') AS email, COUNT(*) AS cnt").
			Joins("LEFT JOIN public.user u ON u.id_user = ae.rid_user").
			Group("ae.rid_user, u.email").
			Order("cnt DESC"),
		f,
	)
	if err := userQ.Scan(&userRows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	byUser := make([]auditReportUserSummary, 0, len(userRows))
	for _, row := range userRows {
		byUser = append(byUser, auditReportUserSummary{
			UserID: row.RIDUser,
			Email:  row.Email,
			Total:  row.Cnt,
		})
	}

	// By day
	type dayRow struct {
		Day string
		Cnt int64
	}
	var dayRows []dayRow
	if err := applyAuditFilters(db.Model(&models.ModelPublicAuditEvent{}), f).
		Select("DATE(datetime) AS day, COUNT(*) AS cnt").
		Group("DATE(datetime)").
		Order("day ASC").
		Scan(&dayRows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	byDay := make([]auditReportDaySummary, 0, len(dayRows))
	for _, row := range dayRows {
		byDay = append(byDay, auditReportDaySummary{Date: row.Day, Total: row.Cnt})
	}

	var fromStr, toStr *string
	if f.from != nil {
		s := f.from.Format(time.RFC3339)
		fromStr = &s
	}
	if f.to != nil {
		s := f.to.Format(time.RFC3339)
		toStr = &s
	}

	writeJSON(w, http.StatusOK, auditReport{
		FromDate:    fromStr,
		ToDate:      toStr,
		TotalEvents: total,
		ByTable:     byTable,
		ByUser:      byUser,
		ByDay:       byDay,
	})
}
