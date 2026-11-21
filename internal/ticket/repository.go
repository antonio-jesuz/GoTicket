package ticket

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type ListFilter struct {
	RequesterID *int64 // preenchido para clientes
	AssigneeID  *int64 // fila "meus chamados"
	Unassigned  bool   // fila "sem agente"
	Status      Status
	Search      string
}

type Repository interface {
	Create(ctx context.Context, t *Ticket) error
	Get(ctx context.Context, id int64) (*Ticket, error)
	List(ctx context.Context, f ListFilter) ([]Ticket, error)
	Update(ctx context.Context, t *Ticket) error
	AddComment(ctx context.Context, c *Comment) error
	Comments(ctx context.Context, ticketID int64, includeInternal bool) ([]Comment, error)
	AddEvent(ctx context.Context, ticketID, actorID int64, kind, detail string) error
	Events(ctx context.Context, ticketID int64) ([]Event, error)
	Parts(ctx context.Context, ticketID int64) ([]Part, error)
	Categories(ctx context.Context) ([]Category, error)
	MarkSLABreached(ctx context.Context) ([]Ticket, error)
}

type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) *PostgresRepository { return &PostgresRepository{db: db} }

const cols = `id, number, title, description, status, priority, requester_id, assignee_id,
	sla_due_at, sla_breached, created_at, updated_at, resolved_at, category_id, asset_id`

type scanner interface{ Scan(dest ...any) error }

func scan(row scanner, t *Ticket) error {
	return row.Scan(&t.ID, &t.Number, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.RequesterID, &t.AssigneeID, &t.SLADueAt, &t.SLABreached, &t.CreatedAt, &t.UpdatedAt,
		&t.ResolvedAt, &t.CategoryID, &t.AssetID)
}

func (r *PostgresRepository) Create(ctx context.Context, t *Ticket) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO tickets (title, description, status, priority, requester_id, sla_due_at, category_id, asset_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, number, created_at, updated_at`,
		t.Title, t.Description, t.Status, t.Priority, t.RequesterID, t.SLADueAt, t.CategoryID, t.AssetID,
	).Scan(&t.ID, &t.Number, &t.CreatedAt, &t.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // FK: categoria ou ativo inexistente
		return fmt.Errorf("%w: categoria ou ativo inexistente", ErrValidation)
	}
	return err
}

func (r *PostgresRepository) Get(ctx context.Context, id int64) (*Ticket, error) {
	var t Ticket
	err := scan(r.db.QueryRowContext(ctx, `SELECT `+cols+` FROM tickets WHERE id = $1`, id), &t)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

func (r *PostgresRepository) List(ctx context.Context, f ListFilter) ([]Ticket, error) {
	q := `SELECT ` + cols + ` FROM tickets WHERE 1=1`
	var args []any
	add := func(clause string, v any) {
		args = append(args, v)
		q += fmt.Sprintf(" AND "+clause, len(args))
	}
	if f.RequesterID != nil {
		add("requester_id = $%d", *f.RequesterID)
	}
	if f.AssigneeID != nil {
		add("assignee_id = $%d", *f.AssigneeID)
	}
	if f.Unassigned {
		q += " AND assignee_id IS NULL"
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Search != "" {
		add("(title ILIKE $%[1]d OR number ILIKE $%[1]d)", "%"+f.Search+"%")
	}
	q += ` ORDER BY created_at DESC LIMIT 200`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		var t Ticket
		if err := scan(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, t *Ticket) error {
	return r.db.QueryRowContext(ctx,
		`UPDATE tickets SET status=$2, assignee_id=$3, resolved_at=$4, updated_at=now()
		 WHERE id=$1 RETURNING updated_at`,
		t.ID, t.Status, t.AssigneeID, t.ResolvedAt).Scan(&t.UpdatedAt)
}

func (r *PostgresRepository) AddComment(ctx context.Context, c *Comment) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO comments (ticket_id, author_id, body, internal) VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at`,
		c.TicketID, c.AuthorID, c.Body, c.Internal).Scan(&c.ID, &c.CreatedAt)
}

func (r *PostgresRepository) Comments(ctx context.Context, ticketID int64, includeInternal bool) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.id, c.ticket_id, c.author_id, u.name, c.body, c.internal, c.created_at
		 FROM comments c JOIN users u ON u.id = c.author_id
		 WHERE c.ticket_id = $1 AND (c.internal = FALSE OR $2) ORDER BY c.created_at`, ticketID, includeInternal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TicketID, &c.AuthorID, &c.AuthorName, &c.Body, &c.Internal, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) AddEvent(ctx context.Context, ticketID, actorID int64, kind, detail string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ticket_events (ticket_id, actor_id, kind, detail) VALUES ($1,$2,$3,$4)`,
		ticketID, actorID, kind, detail)
	return err
}

func (r *PostgresRepository) Events(ctx context.Context, ticketID int64) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, u.name, e.kind, e.detail, e.created_at
		 FROM ticket_events e JOIN users u ON u.id = e.actor_id
		 WHERE e.ticket_id = $1 ORDER BY e.created_at, e.id`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.ActorName, &e.Kind, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Parts(ctx context.Context, ticketID int64) ([]Part, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT i.name, -m.delta, m.created_at
		 FROM stock_movements m JOIN inventory_items i ON i.id = m.item_id
		 WHERE m.ticket_id = $1 AND m.delta < 0 ORDER BY m.created_at`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Part{}
	for rows.Next() {
		var p Part
		if err := rows.Scan(&p.Name, &p.Quantity, &p.At); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Categories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkSLABreached é atômico: o UPDATE ... RETURNING garante que cada violação seja
// devolvida uma única vez, mesmo com mais de uma instância do worker.
func (r *PostgresRepository) MarkSLABreached(ctx context.Context) ([]Ticket, error) {
	rows, err := r.db.QueryContext(ctx,
		`UPDATE tickets SET sla_breached = TRUE, updated_at = now()
		 WHERE sla_breached = FALSE AND sla_due_at < now() AND status NOT IN ('resolved','closed')
		 RETURNING `+cols)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ticket
	for rows.Next() {
		var t Ticket
		if err := scan(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
