package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/real-uangi/allingo/common/business"
)

type sessionPayload struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type SessionClaims struct {
	Username  string
	ExpiresAt time.Time
}

type Service struct {
	cfg *config.AdminAuthConfig
}

func NewService(cfg *config.AdminAuthConfig) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) Login(req dto.AdminLoginRequest) (string, *dto.AdminSessionOutput, error) {
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		return "", nil, business.ErrUnauthorized
	}
	if req.Username != s.cfg.Username || req.Password != s.cfg.Password {
		return "", nil, business.ErrUnauthorized
	}
	claims := SessionClaims{
		Username:  s.cfg.Username,
		ExpiresAt: time.Now().Add(s.cfg.SessionTTL).UTC(),
	}
	token, err := s.sign(claims)
	if err != nil {
		return "", nil, err
	}
	return token, &dto.AdminSessionOutput{
		Username:  claims.Username,
		ExpiresAt: claims.ExpiresAt,
	}, nil
}

func (s *Service) ParseSession(token string) (*SessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, business.ErrUnauthorized
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, business.ErrUnauthorized
	}
	expectedSignature, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, business.ErrUnauthorized
	}
	actualSignature := s.signature(payloadBytes)
	if !hmac.Equal(expectedSignature, actualSignature) {
		return nil, business.ErrUnauthorized
	}
	var payload sessionPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, business.ErrUnauthorized
	}
	if payload.Username == "" || payload.ExpiresAt.IsZero() || time.Now().After(payload.ExpiresAt) {
		return nil, business.ErrUnauthorized
	}
	return &SessionClaims{
		Username:  payload.Username,
		ExpiresAt: payload.ExpiresAt,
	}, nil
}

func (s *Service) sign(claims SessionClaims) (string, error) {
	payloadBytes, err := json.Marshal(sessionPayload{
		Username:  claims.Username,
		ExpiresAt: claims.ExpiresAt,
	})
	if err != nil {
		return "", err
	}
	signature := s.signature(payloadBytes)
	return base64.RawURLEncoding.EncodeToString(payloadBytes) + "." + hex.EncodeToString(signature), nil
}

func (s *Service) signature(payload []byte) []byte {
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	mac.Write(payload)
	return mac.Sum(nil)
}
