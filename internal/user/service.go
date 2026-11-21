package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo   *Repository
	secret []byte
	ttl    time.Duration
}

func NewService(repo *Repository, secret string, ttl time.Duration) *Service {
	return &Service{repo: repo, secret: []byte(secret), ttl: ttl}
}

type tokenClaims struct {
	Role Role `json:"role"`
	jwt.RegisteredClaims
}

// Register cria sempre um Cliente. Agentes só nascem via EnsureAgent (seed/admin).
func (s *Service) Register(ctx context.Context, email, name, password string) (*User, error) {
	return s.create(ctx, email, name, password, RoleCustomer)
}

// EnsureAgent é idempotente: cria o agente inicial se ainda não existir.
func (s *Service) EnsureAgent(ctx context.Context, email, name, password string) error {
	_, err := s.create(ctx, email, name, password, RoleAgent)
	if errors.Is(err, ErrEmailTaken) {
		return nil
	}
	return err
}

func (s *Service) create(ctx context.Context, email, name, password string, role Role) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("%w: e-mail inválido", ErrValidation)
	}
	if name == "" || len(password) < 8 {
		return nil, fmt.Errorf("%w: nome obrigatório e senha com 8+ caracteres", ErrValidation)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &User{Email: email, Name: name, Role: role, PasswordHash: string(hash)}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, *User, error) {
	u, err := s.repo.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrInvalidCredentials
	}
	if err != nil {
		return "", nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", nil, ErrInvalidCredentials
	}
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(u.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	})
	signed, err := tok.SignedString(s.secret)
	return signed, u, err
}

func (s *Service) Parse(token string) (Claims, error) {
	var tc tokenClaims
	_, err := jwt.ParseWithClaims(token, &tc, func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return Claims{}, ErrUnauthorized
	}
	id, err := strconv.ParseInt(tc.Subject, 10, 64)
	if err != nil {
		return Claims{}, ErrUnauthorized
	}
	return Claims{UserID: id, Role: tc.Role}, nil
}

func (s *Service) Me(ctx context.Context, id int64) (*User, error) { return s.repo.ByID(ctx, id) }

func (s *Service) List(ctx context.Context, role Role) ([]User, error) { return s.repo.ListByRole(ctx, role) }

// Name e Agent satisfazem ticket.Directory.
func (s *Service) Name(ctx context.Context, id int64) (string, error) {
	u, err := s.repo.ByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "Usuário removido", nil
	}
	if err != nil {
		return "", err
	}
	return u.Name, nil
}

func (s *Service) Agent(ctx context.Context, id int64) (string, bool, error) {
	u, err := s.repo.ByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return u.Name, u.Role == RoleAgent, nil
}
