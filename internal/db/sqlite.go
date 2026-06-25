package db

import (
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/warkanum/lpwallet/internal/config"
)

// noSchemaDialector wraps a GORM dialector and strips "schema." prefixes from
// all table names before they reach SQLite. This lets GORM models that use
// PostgreSQL schema-qualified names (e.g. "public.account") work transparently
// with SQLite, which doesn't support schemas.
type noSchemaDialector struct {
	gorm.Dialector
}

func (d *noSchemaDialector) QuoteTo(writer clause.Writer, str string) {
	if idx := strings.LastIndex(str, "."); idx >= 0 {
		str = str[idx+1:]
	}
	d.Dialector.QuoteTo(writer, str)
}

// ClauseBuilders forwards any existing clause builders from the inner dialector
// and suppresses FOR UPDATE (clause "FOR"), which SQLite doesn't support.
func (d *noSchemaDialector) ClauseBuilders() map[string]clause.ClauseBuilder {
	type clauseBuilderProvider interface {
		ClauseBuilders() map[string]clause.ClauseBuilder
	}
	builders := make(map[string]clause.ClauseBuilder)
	if cb, ok := d.Dialector.(clauseBuilderProvider); ok {
		for k, v := range cb.ClauseBuilders() {
			builders[k] = v
		}
	}
	builders["FOR"] = func(c clause.Clause, b clause.Builder) {}
	return builders
}

func (d *noSchemaDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return &noSchemaMigrator{Migrator: d.Dialector.Migrator(db), db: db}
}

type noSchemaMigrator struct {
	gorm.Migrator
	db *gorm.DB
}

// FullDataTypeOf maps PostgreSQL serial types to SQLite's integer.
func (m *noSchemaMigrator) FullDataTypeOf(field *schema.Field) clause.Expr {
	expr := m.Migrator.FullDataTypeOf(field)
	switch strings.ToLower(strings.TrimSpace(expr.SQL)) {
	case "bigserial", "serial", "smallserial":
		expr.SQL = "integer"
	}
	return expr
}

func stripSchema(name string) string {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func (m *noSchemaMigrator) HasTable(value interface{}) bool {
	var tableName string
	switch v := value.(type) {
	case string:
		tableName = stripSchema(v)
	default:
		type tabler interface{ TableName() string }
		if t, ok := v.(tabler); ok {
			tableName = stripSchema(t.TableName())
		}
	}
	if tableName == "" {
		return false
	}
	var count int64
	m.db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?", tableName).Scan(&count)
	return count > 0
}

// CreateIndex creates the index with a table-prefixed name to avoid SQLite's
// global index namespace conflicts (PostgreSQL scopes index names per table).
func (m *noSchemaMigrator) CreateIndex(value interface{}, name string) error {
	stmt := &gorm.Statement{DB: m.db}
	if err := stmt.Parse(value); err != nil {
		return err
	}
	tableName := stripSchema(stmt.Table)

	idx := stmt.Schema.LookIndex(name)
	if idx == nil {
		return nil
	}

	cols := make([]string, len(idx.Fields))
	for i, f := range idx.Fields {
		cols[i] = "`" + f.DBName + "`"
	}

	unique := ""
	if idx.Class == "UNIQUE" {
		unique = "UNIQUE "
	}

	return m.db.Exec(fmt.Sprintf(
		"CREATE %sINDEX IF NOT EXISTS `%s_%s` ON `%s` (%s)",
		unique, tableName, name, tableName, strings.Join(cols, ", "),
	)).Error
}

// AutoMigrate creates any missing tables. Column additions on existing tables
// are skipped; delete the SQLite file to apply schema changes in dev.
func (m *noSchemaMigrator) AutoMigrate(values ...interface{}) error {
	for _, value := range values {
		if !m.HasTable(value) {
			if err := m.Migrator.CreateTable(value); err != nil {
				return fmt.Errorf("db: create table: %w", err)
			}
		}
	}
	return nil
}

func openSQLite(cfg *config.Config) (*gorm.DB, error) {
	inner := sqlite.Open(cfg.DBFile)
	db, err := gorm.Open(&noSchemaDialector{Dialector: inner}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("db: open sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	if err := db.Use(&AuditPlugin{}); err != nil {
		return nil, fmt.Errorf("db: register audit plugin: %w", err)
	}
	if err := db.Use(&TransactionPlugin{}); err != nil {
		return nil, fmt.Errorf("db: register transaction plugin: %w", err)
	}

	return db, nil
}
