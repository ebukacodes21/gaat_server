package utils

import (
	"fmt"
	"time"

	"github.com/aead/chacha20poly1305"
	"github.com/o1egl/paseto"
)

type TokenMaker interface {
	CreateToken(email, adminId, role string, duration time.Duration) (string, *Payload, error)
	VerifyToken(token string) (*Payload, error)
}

type Token struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

func NewToken(symmetricKey string) (TokenMaker, error) {
	if len(symmetricKey) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid keysize")
	}

	maker := &Token{
		paseto:       paseto.NewV2(),
		symmetricKey: []byte(symmetricKey),
	}

	return maker, nil
}

func (m *Token) CreateToken(email, adminId, role string, duration time.Duration) (string, *Payload, error) {
	payload, err := NewPayload(email, adminId, role, duration)
	if err != nil {
		return "", payload, err
	}

	token, err := m.paseto.Encrypt(m.symmetricKey, payload, nil)
	return token, payload, err
}

func (m *Token) VerifyToken(token string) (*Payload, error) {
	payload := &Payload{}

	err := m.paseto.Decrypt(token, m.symmetricKey, payload, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}

	err = payload.Valid()
	if err != nil {
		return nil, err
	}

	return payload, nil
}
