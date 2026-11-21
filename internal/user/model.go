// Package user cuida de autenticação (JWT) e perfis (Cliente vs Agente).
package user

import (
	"errors"
	"time"
)

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAgent    Role = "agent"
)

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Claims é a identidade autenticada que trafega no context da requisição.
type Claims struct {
	UserID int64
	Role   Role
}

func (c Claims) IsAgent() bool { return c.Role == RoleAgent }

var (
	ErrInvalidCredentials = errors.New("credenciais inválidas")
	ErrEmailTaken         = errors.New("e-mail já cadastrado")
	ErrValidation         = errors.New("dados inválidos")
	ErrUnauthorized       = errors.New("não autenticado")
)
