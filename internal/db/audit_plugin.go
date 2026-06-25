package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/warkanum/lpwallet/internal/middleware"
	"github.com/warkanum/lpwallet/internal/models"
)

const (
	AuditActionCreate int16 = 1
	AuditActionUpdate int16 = 2
	AuditActionDelete int16 = 3
)

var auditedTables = map[string]bool{
	"user":                true,
	"account":             true,
	"account_transaction": true,
}

type AuditPlugin struct {
	logger  *slog.Logger
	auditDB *gorm.DB
}

func (p *AuditPlugin) Name() string { return "audit_plugin" }

func (p *AuditPlugin) Initialize(db *gorm.DB) error {
	p.auditDB = db.Session(&gorm.Session{NewDB: true, SkipHooks: true})
	if err := db.Callback().Create().After("gorm:create").Register("audit:after_create", p.auditAfterWrite(AuditActionCreate)); err != nil {
		return fmt.Errorf("audit: register create callback: %w", err)
	}
	if err := db.Callback().Update().After("gorm:update").Register("audit:after_update", p.auditAfterWrite(AuditActionUpdate)); err != nil {
		return fmt.Errorf("audit: register update callback: %w", err)
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("audit:after_delete", p.auditAfterWrite(AuditActionDelete)); err != nil {
		return fmt.Errorf("audit: register delete callback: %w", err)
	}
	return nil
}

func (p *AuditPlugin) auditAfterWrite(action int16) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		if tx.Error != nil {
			return
		}
		rawTableName := normalizeAuditTableName(tx.Statement.Table)
		if !auditedTables[rawTableName] {
			return
		}

		var userID sql.NullInt64
		if ctx := tx.Statement.Context; ctx != nil {
			if user, ok := middleware.UserFromContext(ctx); ok {
				userID = sql.NullInt64{Int64: user.IDUser, Valid: true}
			}
		}

		rowID := extractRowID(tx.Statement.Dest)

		event := models.ModelPublicAuditEvent{
			Action:    sql.NullInt16{Int16: action, Valid: true},
			Datetime:  sql.NullTime{Time: time.Now(), Valid: true},
			Tablename: sql.NullString{String: "public." + rawTableName, Valid: true},
			RowID:     rowID,
			RIDUser:   userID,
		}
		fields := extractFields(tx.Statement.Dest)
		p.recordAudit(context.WithoutCancel(tx.Statement.Context), event, fields)

		if p.logger != nil {
			p.logger.Debug("audit: recorded",
				"table", rawTableName,
				"row_id", rowID.Int64,
				"user_id", userID.Int64,
				"action", action,
			)
		}
	}
}

func (p *AuditPlugin) recordAudit(ctx context.Context, event models.ModelPublicAuditEvent, fields map[string]string) {
	auditDB := p.auditDB
	if auditDB == nil {
		return
	}

	go func() {
		const attempts = 10
		if auditDB.Name() == "sqlite" {
			time.Sleep(25 * time.Millisecond)
		}
		for attempt := 1; attempt <= attempts; attempt++ {
			if err := insertAudit(ctx, auditDB, event, fields); err != nil {
				if p.logger != nil {
					p.logger.Error("audit: insert failed", "attempt", attempt, "err", err)
				}
				time.Sleep(time.Duration(attempt) * 25 * time.Millisecond)
				continue
			}
			return
		}
	}()
}

func insertAudit(ctx context.Context, auditDB *gorm.DB, event models.ModelPublicAuditEvent, fields map[string]string) error {
	if err := auditDB.WithContext(ctx).Create(&event).Error; err != nil {
		return fmt.Errorf("event: %w", err)
	}

	for col, val := range fields {
		detail := models.ModelPublicAuditDetail{
			RIDAuditDetail: sql.NullInt64{Int64: event.IDAuditEvent, Valid: true},
			ColumnName:     sql.NullString{String: col, Valid: true},
			ColumnValue:    sql.NullString{String: val, Valid: true},
		}
		if err := auditDB.WithContext(ctx).Create(&detail).Error; err != nil {
			return fmt.Errorf("detail %q: %w", col, err)
		}
	}

	return nil
}

func extractRowID(dest any) sql.NullInt64 {
	if dest == nil {
		return sql.NullInt64{}
	}
	v := reflect.ValueOf(dest)
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return sql.NullInt64{}
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("gorm")
		if strings.Contains(tag, "primaryKey") {
			fv := v.Field(i)
			if fv.Kind() == reflect.Int64 {
				id := fv.Int()
				return sql.NullInt64{Int64: id, Valid: id != 0}
			}
		}
	}
	return sql.NullInt64{}
}

func extractFields(dest any) map[string]string {
	result := make(map[string]string)
	if dest == nil {
		return result
	}
	v := reflect.ValueOf(dest)
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return result
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("gorm")
		if tag == "" || tag == "-" || strings.Contains(tag, "foreignKey") {
			continue
		}
		col := gormColumnName(tag)
		if col == "" {
			continue
		}
		result[col] = fmt.Sprintf("%v", v.Field(i).Interface())
	}
	return result
}

func gormColumnName(tag string) string {
	for _, part := range strings.Split(tag, ";") {
		if after, ok := strings.CutPrefix(part, "column:"); ok {
			return after
		}
	}
	return ""
}

func normalizeAuditTableName(name string) string {
	name = strings.Trim(name, `"`)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}
