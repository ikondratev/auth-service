package token

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ikondratev/auth-service/internal/settings"
)

type Issuer struct {
	key 	  *rsa.PrivateKey
	issuer    string
	audience  string
	scope     string
	accessTTL time.Duration
}

func NewIssuer(s *settings.Settings) (*Issuer, error) {
	auth := s.Auth
	raw, err := os.ReadFile(auth.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("token: read private key: %v", err)
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("token: private key is not Pem")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("token: parse private key error: %w", err)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("toeken: private key is not RSA")
	}

	return &Issuer{
		key:       key,
		issuer:    auth.Issuer,
		audience:  auth.Audience,
		scope:     auth.Scope,
		accessTTL: time.Duration(auth.AccessTTLSec) * time.Second,
	}, nil
}

type Claims struct {
	Email string `json:"email"`
	Scope string `json:"scope"`
	jwt.RegisteredClaims
}

func (i *Issuer) AccessToken(userID, email string) (string, time.Time, error) {
	expiresAt := time.Now().Add(i.accessTTL)
	claims := Claims{
		Email: email,
		Scope: i.scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    i.issuer,
			Audience:  jwt.ClaimStrings{i.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(i.key)
	return signed, expiresAt, err
}