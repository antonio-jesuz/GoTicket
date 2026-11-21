// Package ticket concentra o domínio: estados, prioridades, SLA, service e repository.
package ticket

import (
	"errors"
	"time"
)

type Status string

const (
	StatusNew        Status = "new"
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusPending    Status = "pending"
	StatusResolved   Status = "resolved"
	StatusClosed     Status = "closed"
)

// Máquina de estados: única fonte de verdade sobre transições válidas.
var transitions = map[Status][]Status{
	StatusNew:        {StatusOpen},
	StatusOpen:       {StatusInProgress, StatusPending, StatusResolved},
	StatusInProgress: {StatusOpen, StatusPending, StatusResolved},
	StatusPending:    {StatusInProgress, StatusResolved},
	StatusResolved:   {StatusOpen, StatusClosed}, // reabrir ou encerrar
	StatusClosed:     {},
}

func (s Status) CanMoveTo(to Status) bool {
	for _, next := range transitions[s] {
		if next == to {
			return true
		}
	}
	return false
}

var statusLabels = map[Status]string{
	StatusNew: "Novo", StatusOpen: "Aberto", StatusInProgress: "Em atendimento",
	StatusPending: "Aguardando cliente", StatusResolved: "Resolvido", StatusClosed: "Fechado",
}

func (s Status) Label() string { return statusLabels[s] }

// Active indica se o ticket ainda consome SLA.
func (s Status) Active() bool { return s != StatusResolved && s != StatusClosed }

type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

var slaByPriority = map[Priority]time.Duration{
	PriorityCritical: 4 * time.Hour,
	PriorityHigh:     8 * time.Hour,
	PriorityMedium:   24 * time.Hour,
	PriorityLow:      72 * time.Hour,
}

func (p Priority) Valid() bool { _, ok := slaByPriority[p]; return ok }
func (p Priority) SLA() time.Duration { return slaByPriority[p] }

type Ticket struct {
	ID          int64      `json:"id"`
	Number      string     `json:"number"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	Priority    Priority   `json:"priority"`
	RequesterID int64      `json:"requesterId"`
	AssigneeID  *int64     `json:"assigneeId"`
	SLADueAt    time.Time  `json:"slaDueAt"`
	SLABreached bool       `json:"slaBreached"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt"`
	CategoryID  *int64     `json:"categoryId"`
	AssetID     *int64     `json:"assetId"`
}

type Comment struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticketId"`
	AuthorID  int64     `json:"authorId"`
	AuthorName string   `json:"authorName"`
	Body      string    `json:"body"`
	Internal  bool      `json:"internal"` // nota privada: visível só para agentes
	CreatedAt time.Time `json:"createdAt"`
}

// Event é a linha do tempo (auditoria) do chamado.
type Event struct {
	ID        int64     `json:"id"`
	ActorName string    `json:"actorName"`
	Kind      string    `json:"kind"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Part é uma peça de estoque consumida no atendimento.
type Part struct {
	Name     string    `json:"name"`
	Quantity int       `json:"quantity"`
	At       time.Time `json:"at"`
}

var (
	ErrNotFound          = errors.New("ticket não encontrado")
	ErrForbidden         = errors.New("sem permissão")
	ErrInvalidTransition = errors.New("transição de estado inválida")
	ErrValidation        = errors.New("dados inválidos")
)
