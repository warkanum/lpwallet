package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

func nullStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func nullFloat64(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

func nullInt64(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

func nullInt16(ni sql.NullInt16) *int16 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int16
}

func nullTime(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

func (m ModelPublicUser) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IDUser       int64                     `json:"id_user"`
		ClientID     *string                   `json:"client_id"`
		ClientSecret *string                   `json:"client_secret"`
		Email        *string                   `json:"email"`
		Name         *string                   `json:"name"`
		Password     *string                   `json:"password"`
		Role         *string                   `json:"role"`
		Accounts     []*ModelPublicAccount     `json:"relriduserpublicaccounts,omitempty"`
		AuditEvents  []*ModelPublicAuditEvent  `json:"relriduserpublicauditevents,omitempty"`
		UserSessions []*ModelPublicUserSession `json:"relriduserpublicusersessions,omitempty"`
	}{
		IDUser:       m.IDUser,
		ClientID:     nullStr(m.ClientID),
		ClientSecret: nullStr(m.ClientSecret),
		Email:        nullStr(m.Email),
		Name:         nullStr(m.Name),
		Password:     nullStr(m.Password),
		Role:         nullStr(m.Role),
		Accounts:     m.RelRIDUserPublicAccounts,
		AuditEvents:  m.RelRIDUserPublicAuditEvents,
		UserSessions: m.RelRIDUserPublicUserSessions,
	})
}

func (m ModelPublicAccount) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IDAccount    int64                            `json:"id_account"`
		Balance      *float64                         `json:"balance"`
		Name         *string                          `json:"name"`
		RIDUser      *int64                           `json:"rid_user"`
		User         *ModelPublicUser                 `json:"relriduser,omitempty"`
		Transactions []*ModelPublicAccountTransaction `json:"relridaccountpublicaccounttransactions,omitempty"`
	}{
		IDAccount:    m.IDAccount,
		Balance:      nullFloat64(m.Balance),
		Name:         nullStr(m.Name),
		RIDUser:      nullInt64(m.RIDUser),
		User:         m.RelRIDUser,
		Transactions: m.RelRIDAccountPublicAccountTransactions,
	})
}

func (m ModelPublicAccountTransaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IDAccountTransaction int64                `json:"id_account_transaction"`
		Action               *string              `json:"action"`
		Amount               *float64             `json:"amount"`
		Reference            *string              `json:"reference"`
		RIDAccount           *int64               `json:"rid_account"`
		TransactionDatetime  *time.Time           `json:"transaction_datetime"`
		Account              *ModelPublicAccount  `json:"relridaccount,omitempty"`
	}{
		IDAccountTransaction: m.IDAccountTransaction,
		Action:               nullStr(m.Action),
		Amount:               nullFloat64(m.Amount),
		Reference:            nullStr(m.Reference),
		RIDAccount:           nullInt64(m.RIDAccount),
		TransactionDatetime:  nullTime(m.TransactionDatetime),
		Account:              m.RelRIDAccount,
	})
}

func (m ModelPublicAuditEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IDAuditEvent int64                     `json:"id_audit_event"`
		Action       *int16                    `json:"action"`
		Datetime     *time.Time                `json:"datetime"`
		RIDUser      *int64                    `json:"rid_user"`
		RowID        *int64                    `json:"row_id"`
		Tablename    *string                   `json:"tablename"`
		User         *ModelPublicUser          `json:"relriduser,omitempty"`
		Details      []*ModelPublicAuditDetail `json:"relridauditdetailpublicauditdetails,omitempty"`
	}{
		IDAuditEvent: m.IDAuditEvent,
		Action:       nullInt16(m.Action),
		Datetime:     nullTime(m.Datetime),
		RIDUser:      nullInt64(m.RIDUser),
		RowID:        nullInt64(m.RowID),
		Tablename:    nullStr(m.Tablename),
		User:         m.RelRIDUser,
		Details:      m.RelRIDAuditDetailPublicAuditDetails,
	})
}

func (m ModelPublicAuditDetail) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IDAuditDetail  int64                  `json:"id_audit_detail"`
		ColumnName     *string                `json:"column_name"`
		ColumnValue    *string                `json:"column_value"`
		RIDAuditDetail *int64                 `json:"rid_audit_detail"`
		AuditEvent     *ModelPublicAuditEvent `json:"relridauditdetail,omitempty"`
	}{
		IDAuditDetail:  m.IDAuditDetail,
		ColumnName:     nullStr(m.ColumnName),
		ColumnValue:    nullStr(m.ColumnValue),
		RIDAuditDetail: nullInt64(m.RIDAuditDetail),
		AuditEvent:     m.RelRIDAuditDetail,
	})
}

func (m ModelPublicUserSession) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IDUserSession int64            `json:"id_user_session"`
		Authtoken     *string          `json:"authtoken"`
		Createdat     *time.Time       `json:"createdat"`
		Expiresat     *time.Time       `json:"expiresat"`
		RIDUser       *int64           `json:"rid_user"`
		User          *ModelPublicUser `json:"relriduser,omitempty"`
	}{
		IDUserSession: m.IDUserSession,
		Authtoken:     nullStr(m.Authtoken),
		Createdat:     nullTime(m.Createdat),
		Expiresat:     nullTime(m.Expiresat),
		RIDUser:       nullInt64(m.RIDUser),
		User:          m.RelRIDUser,
	})
}
