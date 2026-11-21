-- Categorias de chamado
CREATE TABLE categories (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
INSERT INTO categories (name) VALUES ('Hardware'), ('Software'), ('Rede'), ('Acesso e contas'), ('Outros');

-- Ativos (CMDB simplificado)
CREATE SEQUENCE asset_tag_seq;
CREATE TABLE assets (
    id             BIGSERIAL PRIMARY KEY,
    tag            TEXT NOT NULL UNIQUE DEFAULT ('AST' || lpad(nextval('asset_tag_seq')::text, 6, '0')),
    name           TEXT NOT NULL,
    type           TEXT NOT NULL CHECK (type IN ('laptop','desktop','monitor','phone','server','printer','network','other')),
    serial         TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'in_stock' CHECK (status IN ('in_stock','in_use','maintenance','retired')),
    assigned_to    BIGINT REFERENCES users(id),
    location       TEXT NOT NULL DEFAULT '',
    warranty_until DATE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_assets_assigned ON assets (assigned_to);
CREATE INDEX idx_assets_status ON assets (status);

-- Estoque de peças e consumíveis
CREATE TABLE inventory_items (
    id           BIGSERIAL PRIMARY KEY,
    sku          TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    category     TEXT NOT NULL DEFAULT '',
    quantity     INT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    min_quantity INT NOT NULL DEFAULT 0 CHECK (min_quantity >= 0),
    location     TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE stock_movements (
    id         BIGSERIAL PRIMARY KEY,
    item_id    BIGINT NOT NULL REFERENCES inventory_items(id),
    delta      INT NOT NULL CHECK (delta <> 0),
    reason     TEXT NOT NULL CHECK (reason IN ('in','out','adjust')),
    ticket_id  BIGINT REFERENCES tickets(id),
    user_id    BIGINT NOT NULL REFERENCES users(id),
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_movements_item ON stock_movements (item_id, created_at DESC);
CREATE INDEX idx_movements_ticket ON stock_movements (ticket_id) WHERE ticket_id IS NOT NULL;

-- Chamado ganha categoria e ativo afetado
ALTER TABLE tickets
    ADD COLUMN category_id INT REFERENCES categories(id),
    ADD COLUMN asset_id    BIGINT REFERENCES assets(id);
CREATE INDEX idx_tickets_asset ON tickets (asset_id) WHERE asset_id IS NOT NULL;

-- Linha do tempo / auditoria do chamado
CREATE TABLE ticket_events (
    id         BIGSERIAL PRIMARY KEY,
    ticket_id  BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    actor_id   BIGINT NOT NULL REFERENCES users(id),
    kind       TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_events_ticket ON ticket_events (ticket_id, created_at);
