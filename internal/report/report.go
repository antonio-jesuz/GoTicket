// Package report calcula os indicadores do painel (dados prontos para gráficos).
package report

import (
	"context"
	"database/sql"
	"net/http"

	"goticket/internal/httpx"
)

type Count struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type Day struct {
	Date     string `json:"date"`
	Created  int    `json:"created"`
	Resolved int    `json:"resolved"`
}

type LowItem struct {
	ID          int64  `json:"id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Quantity    int    `json:"quantity"`
	MinQuantity int    `json:"minQuantity"`
}

type Summary struct {
	Open               int       `json:"open"`
	Unassigned         int       `json:"unassigned"`
	SLABreached        int       `json:"slaBreached"`
	SLACompliance      *float64  `json:"slaCompliance"`      // % resolvidos no prazo, últimos 30 dias
	AvgResolutionHours *float64  `json:"avgResolutionHours"` // últimos 30 dias
	ByStatus           []Count   `json:"byStatus"`
	ByPriority         []Count   `json:"byPriority"`
	ByCategory         []Count   `json:"byCategory"`
	Daily              []Day     `json:"daily"`
	AssetsByStatus     []Count   `json:"assetsByStatus"`
	LowStock           []LowItem `json:"lowStock"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) counts(ctx context.Context, query string) ([]Count, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Count{}
	for rows.Next() {
		var c Count
		if err := rows.Scan(&c.Label, &c.Value); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) Summary(ctx context.Context) (*Summary, error) {
	var sum Summary
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE status NOT IN ('resolved','closed')),
		       count(*) FILTER (WHERE status NOT IN ('resolved','closed') AND assignee_id IS NULL),
		       count(*) FILTER (WHERE status NOT IN ('resolved','closed') AND sla_breached)
		FROM tickets`).Scan(&sum.Open, &sum.Unassigned, &sum.SLABreached)
	if err != nil {
		return nil, err
	}

	var onTime, total int
	var avg sql.NullFloat64
	err = s.db.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE resolved_at <= sla_due_at), count(*),
		       avg(extract(epoch FROM resolved_at - created_at)) / 3600
		FROM tickets WHERE resolved_at IS NOT NULL AND resolved_at > now() - interval '30 days'`,
	).Scan(&onTime, &total, &avg)
	if err != nil {
		return nil, err
	}
	if total > 0 {
		p := float64(onTime) * 100 / float64(total)
		sum.SLACompliance = &p
	}
	if avg.Valid {
		sum.AvgResolutionHours = &avg.Float64
	}

	if sum.ByStatus, err = s.counts(ctx, `SELECT status, count(*) FROM tickets GROUP BY 1 ORDER BY 2 DESC`); err != nil {
		return nil, err
	}
	if sum.ByPriority, err = s.counts(ctx, `SELECT priority, count(*) FROM tickets
		WHERE status NOT IN ('resolved','closed') GROUP BY 1`); err != nil {
		return nil, err
	}
	if sum.ByCategory, err = s.counts(ctx, `SELECT coalesce(c.name, 'Sem categoria'), count(*)
		FROM tickets t LEFT JOIN categories c ON c.id = t.category_id GROUP BY 1 ORDER BY 2 DESC`); err != nil {
		return nil, err
	}
	if sum.AssetsByStatus, err = s.counts(ctx, `SELECT status, count(*) FROM assets GROUP BY 1`); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT to_char(d::date, 'YYYY-MM-DD'),
		       (SELECT count(*) FROM tickets WHERE created_at::date = d::date),
		       (SELECT count(*) FROM tickets WHERE resolved_at::date = d::date)
		FROM generate_series(current_date - 13, current_date, interval '1 day') d ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sum.Daily = []Day{}
	for rows.Next() {
		var d Day
		if err := rows.Scan(&d.Date, &d.Created, &d.Resolved); err != nil {
			return nil, err
		}
		sum.Daily = append(sum.Daily, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	low, err := s.db.QueryContext(ctx, `SELECT id, sku, name, quantity, min_quantity FROM inventory_items
		WHERE quantity <= min_quantity ORDER BY quantity, name LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer low.Close()
	sum.LowStock = []LowItem{}
	for low.Next() {
		var i LowItem
		if err := low.Scan(&i.ID, &i.SKU, &i.Name, &i.Quantity, &i.MinQuantity); err != nil {
			return nil, err
		}
		sum.LowStock = append(sum.LowStock, i)
	}
	return &sum, low.Err()
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, agentOnly func(http.Handler) http.Handler) {
	mux.Handle("GET /api/reports/summary", agentOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sum, err := h.svc.Summary(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "erro interno")
			return
		}
		httpx.JSON(w, http.StatusOK, sum)
	})))
}
