package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenMaker interface {
	CreateToken(email, userId, role string, isAuth bool, duration time.Duration) (string, *Payload, error)
	VerifyToken(token string) (*Payload, error)
}

type Token struct {
	secretKey string
}

func NewToken(secretKey string) (TokenMaker, error) {
	if len(secretKey) < 32 {
		return nil, errors.New("private key must be at least 32 characters")
	}
	return &Token{secretKey: secretKey}, nil

}

func (m *Token) CreateToken(email, userId, role string, isAuth bool, duration time.Duration) (string, *Payload, error) {
	payload, err := NewPayload(email, userId, role, isAuth, duration)
	if err != nil {
		return "", payload, err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	tokenString, err := token.SignedString([]byte(m.secretKey))
	return tokenString, payload, err

}

func (m *Token) VerifyToken(tokenString string) (*Payload, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Payload{}, func(token *jwt.Token) (interface{}, error) {
		// ensure signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(m.secretKey), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	payload, ok := token.Claims.(*Payload)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if err := payload.Valid(); err != nil {
		return nil, err
	}

	return payload, nil
}
