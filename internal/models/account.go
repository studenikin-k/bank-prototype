package models

import "time"

type Account struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Balance   float64   `json:"balance"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateAccountRequest struct {
}

type AccountResponse struct {
	ID        string  `json:"id"`
	Balance   float64 `json:"balance"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

type AccountListResponse struct {
	Accounts      []AccountResponse `json:"accounts"`
	Total         int               `json:"total"`
	ActiveCount   int               `json:"active_count"`
	ClosedCount   int               `json:"closed_count"`
	MaxAccounts   int               `json:"max_accounts"`
	CanCreateMore bool              `json:"can_create_more"`
}
