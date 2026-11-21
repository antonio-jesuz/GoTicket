package ticket

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"goticket/internal/realtime"
	"goticket/internal/user"
)

// Publisher desacopla o domínio do transporte de eventos (realtime.Hub implementa).
type Publisher interface{ Publish(realtime.Event) }

// Directory consulta pessoas sem acoplar o domínio ao pacote user.
type Directory interface {
	Name(ctx context.Context, id int64) (string, error)
	Agent(ctx context.Context, id int64) (name string, ok bool, err error)
}

// Assets consulta ativos sem acoplar o domínio ao pacote asset.
type Assets interface {
	OwnedBy(ctx context.Context, assetID, userID int64) (bool, error)
	Label(ctx context.Context, assetID int64) (string, error)
}

type Service struct {
	repo   Repository
	pub    Publisher
	dir    Directory
	assets Assets
}

func NewService(repo Repository, pub Publisher, dir Directory, assets Assets) *Service {
	return &Service{repo: repo, pub: pub, dir: dir, assets: assets}
}

type CreateInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    Priority `json:"priority"`
	CategoryID  *int64   `json:"categoryId"`
	AssetID     *int64   `json:"assetId"`
}

type Detail struct {
	Ticket        *Ticket   `json:"ticket"`
	Comments      []Comment `json:"comments"`
	Events        []Event   `json:"events"`
	Parts         []Part    `json:"parts"`
	RequesterName string    `json:"requesterName"`
	AssigneeName  string    `json:"assigneeName"`
	CategoryName  string    `json:"categoryName"`
	AssetLabel    string    `json:"assetLabel"`
}

func (s *Service) event(ctx context.Context, ticketID, actorID int64, kind, detail string) {
	if err := s.repo.AddEvent(ctx, ticketID, actorID, kind, detail); err != nil {
		slog.Error("falha ao registrar evento do chamado", "ticket", ticketID, "err", err)
	}
}

func (s *Service) Create(ctx context.Context, actor user.Claims, in CreateInput) (*Ticket, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || len(in.Title) > 200 {
		return nil, fmt.Errorf("%w: título obrigatório (até 200 caracteres)", ErrValidation)
	}
	if in.Priority == "" {
		in.Priority = PriorityMedium
	}
	if !in.Priority.Valid() {
		return nil, fmt.Errorf("%w: prioridade inválida", ErrValidation)
	}
	// Cliente só pode vincular ativos que estão sob a sua responsabilidade.
	if in.AssetID != nil && !actor.IsAgent() {
		ok, err := s.assets.OwnedBy(ctx, *in.AssetID, actor.UserID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%w: ativo inválido", ErrValidation)
		}
	}
	t := &Ticket{
		Title: in.Title, Description: strings.TrimSpace(in.Description),
		Status: StatusNew, Priority: in.Priority, RequesterID: actor.UserID,
		SLADueAt:   time.Now().Add(in.Priority.SLA()),
		CategoryID: in.CategoryID, AssetID: in.AssetID,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	s.event(ctx, t.ID, actor.UserID, "created", "Chamado aberto")
	s.pub.Publish(realtime.Event{Type: "ticket.created", TicketID: t.ID, OwnerID: t.RequesterID, Data: t})
	return t, nil
}

type ListQuery struct {
	Status   Status
	Search   string
	Assignee string // "me" | "none" | ""
}

func (s *Service) List(ctx context.Context, actor user.Claims, q ListQuery) ([]Ticket, error) {
	f := ListFilter{Status: q.Status, Search: strings.TrimSpace(q.Search)}
	if !actor.IsAgent() {
		f.RequesterID = &actor.UserID
	} else {
		switch q.Assignee {
		case "me":
			f.AssigneeID = &actor.UserID
		case "none":
			f.Unassigned = true
		}
	}
	return s.repo.List(ctx, f)
}

func (s *Service) Categories(ctx context.Context) ([]Category, error) { return s.repo.Categories(ctx) }

// load aplica a regra de visibilidade: cliente só enxerga os próprios tickets.
// Responde ErrNotFound (e não Forbidden) para não vazar a existência do ticket.
func (s *Service) load(ctx context.Context, actor user.Claims, id int64) (*Ticket, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.IsAgent() && t.RequesterID != actor.UserID {
		return nil, ErrNotFound
	}
	return t, nil
}

func (s *Service) Get(ctx context.Context, actor user.Claims, id int64) (*Detail, error) {
	t, err := s.load(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	d := &Detail{Ticket: t}
	if d.Comments, err = s.repo.Comments(ctx, id, actor.IsAgent()); err != nil {
		return nil, err
	}
	if d.Events, err = s.repo.Events(ctx, id); err != nil {
		return nil, err
	}
	if d.Parts, err = s.repo.Parts(ctx, id); err != nil {
		return nil, err
	}
	d.RequesterName, _ = s.dir.Name(ctx, t.RequesterID)
	if t.AssigneeID != nil {
		d.AssigneeName, _ = s.dir.Name(ctx, *t.AssigneeID)
	}
	if t.CategoryID != nil {
		if cats, err := s.repo.Categories(ctx); err == nil {
			for _, c := range cats {
				if c.ID == *t.CategoryID {
					d.CategoryName = c.Name
				}
			}
		}
	}
	if t.AssetID != nil {
		d.AssetLabel, _ = s.assets.Label(ctx, *t.AssetID)
	}
	return d, nil
}

func (s *Service) Transition(ctx context.Context, actor user.Claims, id int64, to Status) (*Ticket, error) {
	if !actor.IsAgent() {
		return nil, ErrForbidden
	}
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !t.Status.CanMoveTo(to) {
		return nil, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, t.Status, to)
	}
	from := t.Status
	t.Status = to
	switch to {
	case StatusResolved:
		now := time.Now()
		t.ResolvedAt = &now
	case StatusOpen:
		t.ResolvedAt = nil // reabertura
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	s.event(ctx, t.ID, actor.UserID, "status", from.Label()+" → "+to.Label())
	s.pub.Publish(realtime.Event{Type: "ticket.updated", TicketID: t.ID, OwnerID: t.RequesterID, Data: t})
	return t, nil
}

func (s *Service) Assign(ctx context.Context, actor user.Claims, id, assigneeID int64) (*Ticket, error) {
	if !actor.IsAgent() {
		return nil, ErrForbidden
	}
	name, ok, err := s.dir.Agent(ctx, assigneeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: o responsável precisa ser um agente", ErrValidation)
	}
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	t.AssigneeID = &assigneeID
	if t.Status == StatusNew {
		t.Status = StatusOpen
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	s.event(ctx, t.ID, actor.UserID, "assigned", "Atribuído a "+name)
	s.pub.Publish(realtime.Event{Type: "ticket.updated", TicketID: t.ID, OwnerID: t.RequesterID, Data: t})
	return t, nil
}

func (s *Service) AddComment(ctx context.Context, actor user.Claims, id int64, body string, internal bool) (*Comment, error) {
	t, err := s.load(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("%w: comentário vazio", ErrValidation)
	}
	if internal && !actor.IsAgent() {
		return nil, ErrForbidden
	}
	c := &Comment{TicketID: id, AuthorID: actor.UserID, Body: body, Internal: internal}
	if err := s.repo.AddComment(ctx, c); err != nil {
		return nil, err
	}
	s.pub.Publish(realtime.Event{Type: "comment.added", TicketID: id, OwnerID: t.RequesterID, Internal: internal, Data: c})
	return c, nil
}
