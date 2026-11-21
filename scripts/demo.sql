-- Dados de demonstração. Rode DEPOIS de subir a API uma vez (ela cria o agente e as tabelas):
--   make demo
-- Senhas: clientes "cliente123", agente extra "agente123".
CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
DECLARE agent1 BIGINT; agent2 BIGINT; c1 BIGINT; c2 BIGINT;
BEGIN
  IF EXISTS (SELECT 1 FROM users WHERE email = 'cliente@goticket.local') THEN
    RAISE NOTICE 'Demo já carregada; nada a fazer.';
    RETURN;
  END IF;

  INSERT INTO users (email, name, role, password_hash) VALUES
    ('cliente@goticket.local', 'Camila Duarte', 'customer', crypt('cliente123', gen_salt('bf', 10))),
    ('joao@goticket.local',    'João Pereira',  'customer', crypt('cliente123', gen_salt('bf', 10))),
    ('agente2@goticket.local', 'Rafael Souza',  'agent',    crypt('agente123', gen_salt('bf', 10)));
  SELECT id INTO c1 FROM users WHERE email = 'cliente@goticket.local';
  SELECT id INTO c2 FROM users WHERE email = 'joao@goticket.local';
  SELECT id INTO agent2 FROM users WHERE email = 'agente2@goticket.local';
  SELECT id INTO agent1 FROM users WHERE role = 'agent' ORDER BY id LIMIT 1;

  INSERT INTO assets (name, type, serial, status, assigned_to, location, warranty_until) VALUES
    ('Dell Latitude 5440', 'laptop',  'DL5440-A1', 'in_use',      c1,   'São Paulo', current_date + 400),
    ('Lenovo ThinkPad T14', 'laptop', 'TP14-B7',   'in_use',      c2,   'São Paulo', current_date + 250),
    ('Dell UltraSharp 24', 'monitor', 'US24-091',  'in_use',      c1,   'São Paulo', current_date + 120),
    ('HP LaserJet M404',   'printer', 'HPM404-77', 'in_use',      NULL, '2º andar',  current_date + 60),
    ('Switch Cisco 2960',  'network', 'CS2960-5',  'in_use',      NULL, 'Sala de TI', current_date + 700),
    ('Servidor Dell R740', 'server',  'R740-001',  'in_use',      NULL, 'Datacenter', current_date + 900),
    ('MacBook Air M2',     'laptop',  'MBA-M2-14', 'in_stock',    NULL, 'Almoxarifado', current_date + 500),
    ('Lenovo ThinkPad E14','laptop',  'TPE14-02',  'maintenance', NULL, 'Bancada de TI', current_date - 30),
    ('iPhone 13',          'phone',   'IP13-883',  'retired',     NULL, 'Almoxarifado', current_date - 200);

  INSERT INTO inventory_items (sku, name, category, quantity, min_quantity, location) VALUES
    ('MOUSE-USB',  'Mouse USB',               'Periféricos', 24, 10, 'Prateleira A1'),
    ('KB-ABNT',    'Teclado ABNT2',           'Periféricos', 3,  8,  'Prateleira A1'),
    ('SSD-480',    'SSD SATA 480 GB',         'Componentes', 6,  5,  'Prateleira B2'),
    ('RAM-16-D4',  'Memória DDR4 16 GB',      'Componentes', 2,  4,  'Prateleira B2'),
    ('CAB-HDMI',   'Cabo HDMI 2 m',           'Cabos',       30, 10, 'Gaveta C'),
    ('CAB-RJ45',   'Cabo de rede Cat6 3 m',   'Cabos',       18, 15, 'Gaveta C'),
    ('TONER-M404', 'Toner HP M404',           'Consumíveis', 1,  2,  'Armário D'),
    ('PSU-NB-65',  'Fonte notebook 65 W',     'Componentes', 7,  3,  'Prateleira B1');

  INSERT INTO tickets (title, description, status, priority, requester_id, assignee_id, category_id,
                       sla_due_at, sla_breached, created_at, updated_at, resolved_at)
  SELECT b.title, 'Chamado de demonstração.', b.status, b.priority,
         CASE WHEN b.g % 2 = 0 THEN c1 ELSE c2 END,
         CASE WHEN b.status = 'new' THEN NULL WHEN b.g % 3 = 0 THEN agent2 ELSE agent1 END,
         (SELECT id FROM categories ORDER BY id OFFSET (b.g % 5) LIMIT 1),
         b.created + CASE b.priority WHEN 'critical' THEN interval '4 hours' WHEN 'high' THEN interval '8 hours'
                                     WHEN 'medium' THEN interval '24 hours' ELSE interval '72 hours' END,
         false, b.created, b.created, NULL
  FROM (
    SELECT g,
      (ARRAY['Notebook não liga','Sem acesso à VPN','Instalar pacote Office','Impressora offline',
             'Resetar senha do e-mail','Mouse com defeito','Wi-Fi instável','Erro ao abrir o ERP'])[1 + g % 8] AS title,
      (ARRAY['low','medium','medium','high','critical'])[1 + (g * 7) % 5] AS priority,
      (ARRAY['new','open','in_progress','pending','resolved','resolved','closed'])[1 + (g * 3) % 7] AS status,
      now() - (random() * 13 || ' days')::interval - (random() * 20 || ' hours')::interval AS created
    FROM generate_series(1, 40) g
  ) b;

  -- fecha os chamados resolvidos/fechados e marca SLA conforme o tempo de resolução
  UPDATE tickets
     SET resolved_at = LEAST(now(), created_at + (random() * 30 || ' hours')::interval)
   WHERE status IN ('resolved', 'closed') AND resolved_at IS NULL;
  UPDATE tickets SET sla_breached = (resolved_at > sla_due_at) WHERE resolved_at IS NOT NULL;
  UPDATE tickets SET sla_breached = TRUE
   WHERE resolved_at IS NULL AND sla_due_at < now() - interval '1 hour'; -- o worker marca o restante

  INSERT INTO ticket_events (ticket_id, actor_id, kind, detail, created_at)
  SELECT id, requester_id, 'created', 'Chamado aberto', created_at FROM tickets WHERE description = 'Chamado de demonstração.';
END $$;
