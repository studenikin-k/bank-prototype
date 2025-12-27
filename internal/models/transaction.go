package models

import "time"

type Transaction struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	FromAccountID string    `json:"from_account_id"`
	ToAccountID   string    `json:"to_account_id"`
	Amount        float64   `json:"amount"`
	FeePercent    int       `json:"fee_percent"`
	FeeAmount     float64   `json:"fee_amount"`
	TotalDebit    float64   `json:"total_debit"`
	FeeAccountID  string    `json:"fee_account_id"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type TransferRequest struct {
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        float64 `json:"amount"`
}

type PaymentRequest struct {
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        float64 `json:"amount"`
}

type TransactionRequest struct {
	Type          string  `json:"type"` // "transfer" или "payment"
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        float64 `json:"amount"`
}

type TransactionResponse struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"`
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        float64 `json:"amount"`
	FeePercent    int     `json:"fee_percent"`
	FeeAmount     float64 `json:"fee_amount"`
	TotalDebit    float64 `json:"total_debit"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

type TransactionListResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	Total        int                   `json:"total"`
	AccountID    string                `json:"account_id,omitempty"`
}
