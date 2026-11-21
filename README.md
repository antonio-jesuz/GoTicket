# GoTicket

Sistema de chamados (ITSM) inspirado no ServiceNow: tickets com ciclo de vida controlado, SLA por prioridade,
categorias, filas, **ativos**, **controle de estoque**, **painel com gráficos**, perfis Cliente/Agente e tempo real.

**Stack:** Go 1.23 (stdlib `net/http`) · PostgreSQL · JWT · SSE · Vue 3 + Vite + Pinia · Mailpit (e-mail em dev).

## Subindo em desenvolvimento

```bash
cp .env.example .env
make up          # Postgres + Mailpit (UI em http://localhost:8025)
go mod tidy      # gera go.sum na primeira vez
make run         # API em :8080 (migrações + agente seed automáticos)
make web         # Vue em http://localhost:5173
make demo        # (opcional) dados de demonstração: chamados, ativos, estoque e usuários
```

Agente inicial: `agente@goticket.local` / `agente123` (definido no `.env`).
Clientes se cadastram pela tela de login. Com `make demo` (rode depois de subir a API uma vez) ficam disponíveis também
`cliente@goticket.local` e `joao@goticket.local` (senha `cliente123`) e `agente2@goticket.local` (senha `agente123`).

## Telas

| Tela | Quem | O que faz |
|---|---|---|
| Painel | agente | KPIs, chamados por dia, status, prioridade, categoria, ativos por status, estoque baixo |
| Chamados | todos | busca, filas (meus / sem agente), categoria, ativo afetado, SLA |
| Detalhe do chamado | todos | conversa, notas internas, atribuição, transições, peças usadas, histórico |
| Ativos | agente | inventário, status, responsável, garantia |
| Estoque | agente | saldo, mínimo, entradas/saídas/ajustes, histórico de movimentações |

## API

| Método | Rota | Quem |
|---|---|---|
| POST | `/api/auth/register`, `/api/auth/login` | público |
| GET | `/api/me` | autenticado |
| GET/POST | `/api/tickets` | cliente (só os seus) · agente (todos) |
| GET | `/api/tickets/{id}` | idem (notas internas só para agente) |
| POST | `/api/tickets/{id}/transition` `{to}` | agente |
| POST | `/api/tickets/{id}/assign` `{assigneeId}` | agente |
| POST | `/api/tickets/{id}/comments` `{body, internal}` | idem |
| GET | `/api/categories` | autenticado |
| GET | `/api/users?role=agent` | agente |
| GET | `/api/tickets?status=&q=&assignee=me\|none` | filtros de fila (assignee só para agente) |
| GET/POST | `/api/assets` | agente vê todos e cadastra · cliente vê só os seus |
| POST | `/api/assets/{id}/assign` `{userId\|null}`, `/api/assets/{id}/status` `{status}` | agente |
| GET/POST | `/api/inventory` | agente |
| GET/POST | `/api/inventory/{id}/movements` `{reason: in\|out\|adjust, quantity, ticketId?, note?}` | agente |
| GET | `/api/reports/summary` | agente |
| GET | `/api/events` (SSE) | autenticado, filtrado por perfil |

## Documentação estratégica

Veja [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md): visão, decisões (ADRs), modelo de domínio,
estratégia de frontend, segurança, riscos e roadmap.

> Nota: o código foi escrito sem acesso a toolchain Go/Node no ambiente de geração, então ele
> ainda não foi compilado. Rode `go mod tidy && go vet ./... && go test ./...` primeiro.
