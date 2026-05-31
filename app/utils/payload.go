package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("token is invalid")
	ErrExpiredToken = errors.New("token is expired, logout to get a fresh token")
)

type Payload struct {
	ID     uuid.UUID `json:"id"`
	Email  string    `json:"email"`
	UserId string    `json:"userId"`
	IsAuth bool      `json:"isAuth"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// instantiate a payload
func NewPayload(email, userId, role string, isAuth bool, duration time.Duration) (*Payload, error) {
	tokenId, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	payload := &Payload{
		ID:     tokenId,
		Email:  email,
		UserId: userId,
		IsAuth: isAuth,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			ID:        tokenId.String(),
		},
	}

	return payload, nil
}

func (p *Payload) Valid() error {
	if p.ExpiresAt == nil || time.Now().After(p.ExpiresAt.Time) {
		return ErrExpiredToken
	}
	return nil
}
