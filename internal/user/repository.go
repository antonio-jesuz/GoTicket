package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const userCols = `id, email, name, role, password_hash, created_at`

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	return &u, err
}

func (r *Repository) Create(ctx context.Context, u *User) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (email, name, role, password_hash) VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at`,
		u.Email, u.Name, u.Role, u.PasswordHash).Scan(&u.ID, &u.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailTaken
	}
	return err
}

func (r *Repository) ByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE email = $1`, email))
}

func (r *Repository) ByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

func (r *Repository) ListByRole(ctx context.Context, role Role) ([]User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+userCols+` FROM users WHERE ($1 = '' OR role = $1) ORDER BY name`, string(role))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
