.PHONY: up down run web test demo
up:   ; docker compose up -d
down: ; docker compose down
run:  ; set -a; . ./.env; set +a; go run ./cmd/api
web:  ; cd web && npm install && npm run dev
test: ; go test ./...
demo: ; docker compose exec -T postgres psql -U goticket -d goticket -v ON_ERROR_STOP=1 < scripts/demo.sql
