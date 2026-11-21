// Package asset gerencia ativos de TI (notebooks, monitores, servidores...) e quem os utiliza.
package asset

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
	"goticket/internal/user"
)

type Asset struct {
	ID            int64      `json:"id"`
	Tag           string     `json:"tag"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`
	Serial        string     `json:"serial"`
	Status        string     `json:"status"`
	AssignedTo    *int64     `json:"assignedTo"`
	AssignedName  *string    `json:"assignedName"`
	Location      string     `json:"location"`
	WarrantyUntil *time.Time `json:"warrantyUntil"`
	CreatedAt     time.Time  `json:"createdAt"`
}

var (
	ErrNotFound   = errors.New("ativo não encontrado")
	ErrValidation = errors.New("dados inválidos")
	types         = map[string]bool{"laptop": true, "desktop": true, "monitor": true, "phone": true, "server": true, "printer": true, "network": true, "other": true}
	statuses      = map[string]bool{"in_stock": true, "in_use": true, "maintenance": true, "retired": true}
)

// ---------- repository ----------

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const selectAsset = `SELECT a.id, a.tag, a.name, a.type, a.serial, a.status, a.assigned_to, u.name,
	a.location, a.warranty_until, a.created_at
	FROM assets a LEFT JOIN users u ON u.id = a.assigned_to`

type scanner interface{ Scan(...any) error }

func scanAsset(s scanner, a *Asset) error {
	return s.Scan(&a.ID, &a.Tag, &a.Name, &a.Type, &a.Serial, &a.Status, &a.AssignedTo, &a.AssignedName,
		&a.Location, &a.WarrantyUntil, &a.CreatedAt)
}

func (r *Repository) Get(ctx context.Context, id int64) (*Asset, error) {
	var a Asset
	err := scanAsset(r.db.QueryRowContext(ctx, selectAsset+` WHERE a.id = $1`, id), &a)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

func (r *Repository) List(ctx context.Context, status, q string, assignedTo *int64) ([]Asset, error) {
	query := selectAsset + ` WHERE 1=1`
	var args []any
	add := func(clause string, v any) {
		args = append(args, v)
		query += fmt.Sprintf(" AND "+clause, len(args))
	}
	if status != "" {
		add("a.status = $%d", status)
	}
	if assignedTo != nil {
		add("a.assigned_to = $%d", *assignedTo)
	}
	if q != "" {
		add("(a.name ILIKE $%[1]d OR a.tag ILIKE $%[1]d OR a.serial ILIKE $%[1]d)", "%"+q+"%")
	}
	rows, err := r.db.QueryContext(ctx, query+` ORDER BY a.id DESC LIMIT 300`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Asset{}
	for rows.Next() {
		var a Asset
		if err := scanAsset(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) Create(ctx context.Context, a *Asset) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO assets (name, type, serial, status, location, warranty_until)
		 VALUES ($1,$2,$3,'in_stock',$4,$5) RETURNING id, tag, status, created_at`,
		a.Name, a.Type, a.Serial, a.Location, a.WarrantyUntil).Scan(&a.ID, &a.Tag, &a.Status, &a.CreatedAt)
}

func (r *Repository) Update(ctx context.Context, a *Asset) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assets SET status=$2, assigned_to=$3, location=$4, updated_at=now() WHERE id=$1`,
		a.ID, a.Status, a.AssignedTo, a.Location)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return fmt.Errorf("%w: usuário inexistente", ErrValidation)
	}
	return err
}

// ---------- service ----------

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

type CreateInput struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Serial        string `json:"serial"`
	Location      string `json:"location"`
	WarrantyUntil string `json:"warrantyUntil"` // AAAA-MM-DD
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Asset, error) {
	a := &Asset{Name: strings.TrimSpace(in.Name), Type: in.Type, Serial: strings.TrimSpace(in.Serial), Location: strings.TrimSpace(in.Location)}
	if a.Name == "" || !types[a.Type] {
		return nil, fmt.Errorf("%w: nome e tipo válidos são obrigatórios", ErrValidation)
	}
	if in.WarrantyUntil != "" {
		d, err := time.Parse("2006-01-02", in.WarrantyUntil)
		if err != nil {
			return nil, fmt.Errorf("%w: data de garantia inválida", ErrValidation)
		}
		a.WarrantyUntil = &d
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// List: agente vê tudo; cliente vê apenas os ativos sob sua responsabilidade.
func (s *Service) List(ctx context.Context, actor user.Claims, status, q string) ([]Asset, error) {
	var owner *int64
	if !actor.IsAgent() {
		owner = &actor.UserID
	}
	return s.repo.List(ctx, status, strings.TrimSpace(q), owner)
}

// Assign entrega (ou devolve, com userID nulo) um ativo; ajusta o status coerentemente.
func (s *Service) Assign(ctx context.Context, id int64, userID *int64) (*Asset, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	a.AssignedTo = userID
	switch {
	case userID != nil && (a.Status == "in_stock" || a.Status == "maintenance"):
		a.Status = "in_use"
	case userID == nil && a.Status == "in_use":
		a.Status = "in_stock"
	}
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) SetStatus(ctx context.Context, id int64, status string) (*Asset, error) {
	if !statuses[status] {
		return nil, fmt.Errorf("%w: status inválido", ErrValidation)
	}
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	a.Status = status
	if status == "in_stock" || status == "retired" {
		a.AssignedTo = nil // ativo parado ou baixado não fica com ninguém
	}
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

// OwnedBy e Label satisfazem ticket.Assets.
func (s *Service) OwnedBy(ctx context.Context, assetID, userID int64) (bool, error) {
	a, err := s.repo.Get(ctx, assetID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return a.AssignedTo != nil && *a.AssignedTo == userID, nil
}

func (s *Service) Label(ctx context.Context, assetID int64) (string, error) {
	a, err := s.repo.Get(ctx, assetID)
	if err != nil {
		return "", err
	}
	return a.Tag + " · " + a.Name, nil
}

// ---------- handler ----------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, auth, agentOnly func(http.Handler) http.Handler) {
	mux.Handle("GET /api/assets", auth(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/assets", agentOnly(http.HandlerFunc(h.create)))
	mux.Handle("POST /api/assets/{id}/assign", agentOnly(http.HandlerFunc(h.assign)))
	mux.Handle("POST /api/assets/{id}/status", agentOnly(http.HandlerFunc(h.status)))
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrValidation):
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := user.FromContext(r.Context())
	as, err := h.svc.List(r.Context(), actor, r.URL.Query().Get("status"), r.URL.Query().Get("q"))
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, as)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	a, err := h.svc.Create(r.Context(), in)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var in struct {
		UserID *int64 `json:"userId"` // nulo = devolver ao estoque
	}
	if err != nil || httpx.Decode(w, r, &in) != nil {
		httpx.Error(w, http.StatusBadRequest, "requisição inválida")
		return
	}
	a, err := h.svc.Assign(r.Context(), id, in.UserID)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var in struct {
		Status string `json:"status"`
	}
	if err != nil || httpx.Decode(w, r, &in) != nil {
		httpx.Error(w, http.StatusBadRequest, "requisição inválida")
		return
	}
	a, err := h.svc.SetStatus(r.Context(), id, in.Status)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}
