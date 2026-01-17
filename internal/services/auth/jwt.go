package auth

import (
	"fmt"
	"time"

	"fleetctrl/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// Service handles authentication
type Service struct {
	config *config.Config
}

// Claims represents JWT claims
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewService creates a new auth service
func NewService(cfg *config.Config) *Service {
	return &Service{config: cfg}
}

// Login validates credentials and returns a JWT token
func (s *Service) Login(username, password string) (string, time.Time, error) {
	// Validate username
	if username != s.config.Auth.Username {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}

	// First run: set password if not set
	if s.config.Auth.PasswordHash == "" {
		hash, err := HashPassword(password)
		if err != nil {
			return "", time.Time{}, err
		}
		s.config.Auth.PasswordHash = hash
		if err := s.config.Save(); err != nil {
			return "", time.Time{}, err
		}
	} else {
		// Validate password
		if !CheckPasswordHash(password, s.config.Auth.PasswordHash) {
			return "", time.Time{}, fmt.Errorf("invalid credentials")
		}
	}

	// Generate token
	expiresAt := time.Now().Add(time.Duration(s.config.Auth.JWTExpiryHours) * time.Hour)

	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "fleetctrl",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.Auth.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.Auth.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ChangePassword changes the user password
func (s *Service) ChangePassword(currentPassword, newPassword string) error {
	// Validate current password
	if !CheckPasswordHash(currentPassword, s.config.Auth.PasswordHash) {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	s.config.Auth.PasswordHash = hash
	return s.config.Save()
}

// RefreshToken validates an existing token and issues a new one
func (s *Service) RefreshToken(tokenString string) (string, time.Time, error) {
	// Validate the existing token
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", time.Time{}, err
	}

	// Generate a new token with fresh expiry
	expiresAt := time.Now().Add(time.Duration(s.config.Auth.JWTExpiryHours) * time.Hour)

	newClaims := &Claims{
		Username: claims.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "fleetctrl",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	newTokenString, err := token.SignedString([]byte(s.config.Auth.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return newTokenString, expiresAt, nil
}
