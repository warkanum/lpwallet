package db

import (
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
	"public.user":                true,
	"public.account":             true,
	"public.account_transaction": true,
}

type AuditPlugin struct {
	logger *slog.Logger
}

func (p *AuditPlugin) Name() string { return "audit_plugin" }

func (p *AuditPlugin) Initialize(db *gorm.DB) error {
	db.Callback().Create().After("gorm:create").Register("audit:after_create", p.auditAfterWrite(AuditActionCreate))
	db.Callback().Update().After("gorm:update").Register("audit:after_update", p.auditAfterWrite(AuditActionUpdate))
	db.Callback().Delete().After("gorm:delete").Register("audit:after_delete", p.auditAfterWrite(AuditActionDelete))
	return nil
}

func (p *AuditPlugin) auditAfterWrite(action int16) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		if tx.Error != nil {
			return
		}
		tableName := tx.Statement.Table
		if !auditedTables[tableName] {
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
			Tablename: sql.NullString{String: tableName, Valid: true},
			RowID:     rowID,
			RIDUser:   userID,
		}
		if err := tx.Create(&event).Error; err != nil {
			if p.logger != nil {
				p.logger.Error("audit: insert event failed", "table", tableName, "err", err)
			}
			tx.Error = fmt.Errorf("audit: %w", err)
			return
		}

		fields := extractFields(tx.Statement.Dest)
		for col, val := range fields {
			detail := models.ModelPublicAuditDetail{
				RIDAuditDetail: sql.NullInt64{Int64: event.IDAuditEvent, Valid: true},
				ColumnName:     sql.NullString{String: col, Valid: true},
				ColumnValue:    sql.NullString{String: val, Valid: true},
			}
			if err := tx.Create(&detail).Error; err != nil {
				if p.logger != nil {
					p.logger.Error("audit: insert detail failed", "column", col, "err", err)
				}
				tx.Error = fmt.Errorf("audit: %w", err)
				return
			}
		}

		if p.logger != nil {
			p.logger.Debug("audit: recorded",
				"table", tableName,
				"row_id", rowID.Int64,
				"user_id", userID.Int64,
				"action", action,
			)
		}
	}
}

func extractRowID(dest any) sql.NullInt64 {
	if dest == nil {
		return sql.NullInt64{}
	}
	v := reflect.ValueOf(dest)
	for v.Kind() == reflect.Ptr {
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
	for v.Kind() == reflect.Ptr {
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
