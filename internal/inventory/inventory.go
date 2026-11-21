// Package inventory controla o estoque de peças e consumíveis, com histórico de movimentações
// e vínculo opcional com o chamado em que a peça foi usada.
package inventory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"goticket/internal/httpx"
	"goticket/internal/realtime"
	"goticket/internal/user"
)

type Item struct {
	ID          int64     `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Quantity    int       `json:"quantity"`
	MinQuantity int       `json:"minQuantity"`
	Location    string    `json:"location"`
	Low         bool      `json:"low"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Movement struct {
	ID        int64     `json:"id"`
	Delta     int       `json:"delta"`
	Reason    string    `json:"reason"`
	TicketID  *int64    `json:"ticketId"`
	UserName  string    `json:"userName"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

var (
	ErrNotFound     = errors.New("item não encontrado")
	ErrInsufficient = errors.New("estoque insuficiente")
	ErrValidation   = errors.New("dados inválidos")
	ErrSKUTaken     = errors.New("SKU já cadastrado")
)

// ---------- repository ----------

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const itemCols = `id, sku, name, category, quantity, min_quantity, location, created_at`

type scanner interface{ Scan(...any) error }

func scanItem(s scanner, it *Item) error {
	err := s.Scan(&it.ID, &it.SKU, &it.Name, &it.Category, &it.Quantity, &it.MinQuantity, &it.Location, &it.CreatedAt)
	it.Low = it.Quantity <= it.MinQuantity
	return err
}

func (r *Repository) List(ctx context.Context, q string) ([]Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+itemCols+` FROM inventory_items
		 WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR sku ILIKE '%' || $1 || '%') ORDER BY name`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repository) Create(ctx context.Context, it *Item) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO inventory_items (sku, name, category, quantity, min_quantity, location)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`,
		it.SKU, it.Name, it.Category, it.Quantity, it.MinQuantity, it.Location).Scan(&it.ID, &it.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrSKUTaken
	}
	it.Low = it.Quantity <= it.MinQuantity
	return err
}

// Move aplica a movimentação de forma atômica: o saldo nunca fica negativo, mesmo com
// baixas concorrentes (a condição está no próprio UPDATE).
func (r *Repository) Move(ctx context.Context, itemID int64, delta int, reason string, ticketID *int64, userID int64, note string) (*Item, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var it Item
	err = scanItem(tx.QueryRowContext(ctx,
		`UPDATE inventory_items SET quantity = quantity + $2
		 WHERE id = $1 AND quantity + $2 >= 0 RETURNING `+itemCols, itemID, delta), &it)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		_ = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM inventory_items WHERE id = $1)`, itemID).Scan(&exists)
		if !exists {
			return nil, ErrNotFound
		}
		return nil, ErrInsufficient
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO stock_movements (item_id, delta, reason, ticket_id, user_id, note) VALUES ($1,$2,$3,$4,$5,$6)`,
		itemID, delta, reason, ticketID, userID, note); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, fmt.Errorf("%w: chamado inexistente", ErrValidation)
		}
		return nil, err
	}
	if ticketID != nil && delta < 0 { // registra na linha do tempo do chamado, na mesma transação
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO ticket_events (ticket_id, actor_id, kind, detail) VALUES ($1,$2,'part',$3)`,
			*ticketID, userID, fmt.Sprintf("Peça utilizada: %d× %s", -delta, it.Name)); err != nil {
			return nil, err
		}
	}
	return &it, tx.Commit()
}

func (r *Repository) Movements(ctx context.Context, itemID int64) ([]Movement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT m.id, m.delta, m.reason, m.ticket_id, u.name, m.note, m.created_at
		 FROM stock_movements m JOIN users u ON u.id = m.user_id
		 WHERE m.item_id = $1 ORDER BY m.created_at DESC LIMIT 50`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Movement{}
	for rows.Next() {
		var m Movement
		if err := rows.Scan(&m.ID, &m.Delta, &m.Reason, &m.TicketID, &m.UserName, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------- service ----------

type Publisher interface{ Publish(realtime.Event) }

type Service struct {
	repo *Repository
	pub  Publisher
}

func NewService(repo *Repository, pub Publisher) *Service { return &Service{repo: repo, pub: pub} }

type CreateInput struct {
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Quantity    int    `json:"quantity"`
	MinQuantity int    `json:"minQuantity"`
	Location    string `json:"location"`
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Item, error) {
	it := &Item{SKU: strings.ToUpper(strings.TrimSpace(in.SKU)), Name: strings.TrimSpace(in.Name),
		Category: strings.TrimSpace(in.Category), Quantity: in.Quantity, MinQuantity: in.MinQuantity,
		Location: strings.TrimSpace(in.Location)}
	if it.SKU == "" || it.Name == "" || it.Quantity < 0 || it.MinQuantity < 0 {
		return nil, fmt.Errorf("%w: SKU, nome e quantidades não negativas são obrigatórios", ErrValidation)
	}
	return it, s.repo.Create(ctx, it)
}

type MoveInput struct {
	Reason   string `json:"reason"`   // in (entrada) | out (saída/consumo) | adjust (inventário, +/-)
	Quantity int    `json:"quantity"` // positivo em in/out; com sinal em adjust
	TicketID *int64 `json:"ticketId"` // só em out: peça usada neste chamado
	Note     string `json:"note"`
}

func (s *Service) Move(ctx context.Context, actor user.Claims, itemID int64, in MoveInput) (*Item, error) {
	var delta int
	switch in.Reason {
	case "in":
		delta = in.Quantity
	case "out":
		delta = -in.Quantity
	case "adjust":
		delta = in.Quantity
	default:
		return nil, fmt.Errorf("%w: motivo inválido", ErrValidation)
	}
	if delta == 0 || (in.Reason != "adjust" && in.Quantity < 0) {
		return nil, fmt.Errorf("%w: quantidade inválida", ErrValidation)
	}
	if in.TicketID != nil && in.Reason != "out" {
		return nil, fmt.Errorf("%w: só saídas podem ser vinculadas a um chamado", ErrValidation)
	}
	it, err := s.repo.Move(ctx, itemID, delta, in.Reason, in.TicketID, actor.UserID, strings.TrimSpace(in.Note))
	if err != nil {
		return nil, err
	}
	var tid int64
	if in.TicketID != nil {
		tid = *in.TicketID
	}
	// OwnerID 0: nenhum cliente recebe; agentes recebem tudo.
	s.pub.Publish(realtime.Event{Type: "stock.changed", TicketID: tid, Data: it})
	return it, nil
}

// ---------- handler ----------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, agentOnly func(http.Handler) http.Handler) {
	mux.Handle("GET /api/inventory", agentOnly(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/inventory", agentOnly(http.HandlerFunc(h.create)))
	mux.Handle("POST /api/inventory/{id}/movements", agentOnly(http.HandlerFunc(h.move)))
	mux.Handle("GET /api/inventory/{id}/movements", agentOnly(http.HandlerFunc(h.movements)))
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInsufficient), errors.Is(err, ErrSKUTaken):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrValidation):
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.repo.List(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	it, err := h.svc.Create(r.Context(), in)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, it)
}

func (h *Handler) move(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var in MoveInput
	if err != nil || httpx.Decode(w, r, &in) != nil {
		httpx.Error(w, http.StatusBadRequest, "requisição inválida")
		return
	}
	actor, _ := user.FromContext(r.Context())
	it, err := h.svc.Move(r.Context(), actor, id, in)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, it)
}

func (h *Handler) movements(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return
	}
	ms, err := h.svc.repo.Movements(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ms)
}
