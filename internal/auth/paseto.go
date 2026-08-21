package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/o1egl/paseto"
	"github.com/irvanmalik48/realm-api/internal/model"
)

var (
	ErrInvalidToken = errors.New("invalid or malformed PASETO token")
	ErrExpiredToken = errors.New("PASETO token has expired")
)

type PasetoService interface {
	GenerateToken(user *model.User, duration time.Duration) (string, error)
	VerifyToken(tokenString string) (*model.UserClaims, error)
}

type pasetoService struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

func NewPasetoService(symmetricKeyHex string) (PasetoService, error) {
	var key []byte
	var err error

	if symmetricKeyHex != "" {
		key, err = hex.DecodeString(symmetricKeyHex)
		if err != nil || len(key) != 32 {
			// If not valid 32-byte hex, treat as raw string or pad/truncate
			if len(symmetricKeyHex) == 32 {
				key = []byte(symmetricKeyHex)
			} else {
				// Fallback: derive 32 bytes or generate random key
				key = make([]byte, 32)
				copy(key, []byte(symmetricKeyHex))
			}
		}
	} else {
		// Generate random 32-byte key for local development
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate random paseto key: %w", err)
		}
	}

	return &pasetoService{
		paseto:       paseto.NewV2(),
		symmetricKey: key,
	}, nil
}

func (s *pasetoService) GenerateToken(user *model.User, duration time.Duration) (string, error) {
	now := time.Now().UTC()
	exp := now.Add(duration)

	avatar := ""
	if user.AvatarURL != nil {
		avatar = *user.AvatarURL
	}

	claims := model.UserClaims{
		ID:        user.ID.String(),
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		AvatarURL: avatar,
		Provider:  user.Provider,
		Issuer:    "realm-api",
		Subject:   user.ID.String(),
		Audience:  "realm-frontend",
		IssuedAt:  now,
		ExpiresAt: exp,
	}

	token, err := s.paseto.Encrypt(s.symmetricKey, claims, nil)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt paseto token: %w", err)
	}

	return token, nil
}

func (s *pasetoService) VerifyToken(tokenString string) (*model.UserClaims, error) {
	var claims model.UserClaims
	var footer string

	err := s.paseto.Decrypt(tokenString, s.symmetricKey, &claims, &footer)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().UTC().After(claims.ExpiresAt) {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}
