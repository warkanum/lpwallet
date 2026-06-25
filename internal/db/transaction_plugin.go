package db

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/warkanum/lpwallet/internal/models"
)

var (
	ErrDuplicateTransaction = errors.New("transaction reference already captured for this account")
	ErrInsufficientBalance  = errors.New("spend would drive balance below zero")
)

type TransactionPlugin struct{}

func (p *TransactionPlugin) Name() string { return "transaction_plugin" }

func (p *TransactionPlugin) Initialize(db *gorm.DB) error {
	db.Callback().Create().Before("gorm:create").Register("txn:before_create", beforeCreateTransaction)
	return nil
}

func beforeCreateTransaction(tx *gorm.DB) {
	t := tx.Statement.Table
	if t != "public.account_transaction" && t != "account_transaction" {
		return
	}

	record, ok := tx.Statement.Dest.(*models.ModelPublicAccountTransaction)
	if !ok {
		return
	}
	if !record.RIDAccount.Valid || !record.Reference.Valid {
		return
	}

	sub := tx.Session(&gorm.Session{NewDB: true})

	// Check 1: duplicate (reference, rid_account)
	var count int64
	sub.Model(&models.ModelPublicAccountTransaction{}).
		Where("reference = ? AND rid_account = ?", record.Reference.String, record.RIDAccount.Int64).
		Count(&count)
	if sub.Error != nil {
		tx.AddError(sub.Error)
		return
	}
	if count > 0 {
		tx.AddError(ErrDuplicateTransaction)
		return
	}

	// Check 2: spend cannot drive balance below zero
	if record.Action.Valid && record.Action.String == "spend" {
		var account models.ModelPublicAccount
		sub.Session(&gorm.Session{NewDB: true}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&account, record.RIDAccount.Int64)
		if sub.Error != nil {
			tx.AddError(sub.Error)
			return
		}
		current := 0.0
		if account.Balance.Valid {
			current = account.Balance.Float64
		}
		if current-record.Amount.Float64 < 0 {
			tx.AddError(ErrInsufficientBalance)
		}
	}
}
