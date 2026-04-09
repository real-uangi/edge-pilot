package dto

import "time"

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AdminSessionOutput struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expiresAt"`
}
