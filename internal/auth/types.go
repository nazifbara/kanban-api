package auth

import "github.com/google/uuid"

type Params struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type IdentityWithToken struct {
	ID           uuid.UUID `json:"id"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}
