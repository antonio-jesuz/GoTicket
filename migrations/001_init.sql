CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('customer', 'agent')),
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE SEQUENCE ticket_number_seq;

CREATE TABLE tickets (
    id           BIGSERIAL PRIMARY KEY,
    number       TEXT NOT NULL UNIQUE
                 DEFAULT ('INC' || lpad(nextval('ticket_number_seq')::text, 7, '0')),
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'new'
                 CHECK (status IN ('new','open','in_progress','pending','resolved','closed')),
    priority     TEXT NOT NULL CHECK (priority IN ('critical','high','medium','low')),
    requester_id BIGINT NOT NULL REFERENCES users(id),
    assignee_id  BIGINT REFERENCES users(id),
    sla_due_at   TIMESTAMPTZ NOT NULL,
    sla_breached BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at  TIMESTAMPTZ
);

CREATE INDEX idx_tickets_requester ON tickets (requester_id);
CREATE INDEX idx_tickets_status ON tickets (status);
-- índice parcial: o worker de SLA só varre tickets ainda "vivos" e não violados
CREATE INDEX idx_tickets_sla_pending ON tickets (sla_due_at)
    WHERE sla_breached = FALSE AND status NOT IN ('resolved','closed');

CREATE TABLE comments (
    id         BIGSERIAL PRIMARY KEY,
    ticket_id  BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id  BIGINT NOT NULL REFERENCES users(id),
    body       TEXT NOT NULL,
    internal   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_comments_ticket ON comments (ticket_id, created_at);
