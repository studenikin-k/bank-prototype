package models

import "time"

type User struct {
	ID           string
	Name         string
	PasswordHash string
	CreatedAt    time.Time
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}
