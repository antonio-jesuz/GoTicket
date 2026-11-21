// Ponto de entrada: composição de dependências e graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"goticket/internal/asset"
	"goticket/internal/database"
	"goticket/internal/inventory"
	"goticket/internal/realtime"
	"goticket/internal/report"
	"goticket/internal/ticket"
	"goticket/internal/user"
	"goticket/internal/worker"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	if err := run(); err != nil {
		slog.Error("falha fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return errors.New("JWT_SECRET deve ter 32+ caracteres")
	}

	// Infra
	db, err := database.Open(ctx, env("DATABASE_URL", "postgres://goticket:goticket@localhost:5432/goticket?sslmode=disable"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := database.Migrate(ctx, db, env("MIGRATIONS_DIR", "migrations")); err != nil {
		return err
	}

	// Composição (repository → service → handler)
	hub := realtime.NewHub()
	userSvc := user.NewService(user.NewRepository(db), secret, 12*time.Hour)
	if email := os.Getenv("SEED_AGENT_EMAIL"); email != "" {
		if err := userSvc.EnsureAgent(ctx, email, "Agente", os.Getenv("SEED_AGENT_PASSWORD")); err != nil {
			return err
		}
	}
	assetSvc := asset.NewService(asset.NewRepository(db))
	ticketRepo := ticket.NewPostgresRepository(db)
	ticketSvc := ticket.NewService(ticketRepo, hub, userSvc, assetSvc)
	inventorySvc := inventory.NewService(inventory.NewRepository(db), hub)
	reportSvc := report.NewService(db)

	auth := userSvc.Authenticate
	agentOnly := func(h http.Handler) http.Handler { return auth(user.RequireAgent(h)) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "db indisponível", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	user.NewHandler(userSvc).Register(mux, agentOnly)
	ticket.NewHandler(ticketSvc).Register(mux, auth)
	asset.NewHandler(assetSvc).Register(mux, auth, agentOnly)
	inventory.NewHandler(inventorySvc).Register(mux, agentOnly)
	report.NewHandler(reportSvc).Register(mux, agentOnly)
	mux.Handle("GET /api/events", auth(http.HandlerFunc(hub.ServeSSE)))

	srv := &http.Server{
		Addr:              env("HTTP_ADDR", ":8080"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		// sem WriteTimeout global: conexões SSE são longas
	}

	// Worker em background, atrelado ao mesmo ctx
	var wg sync.WaitGroup
	notifier := worker.SMTPNotifier{
		Addr: env("SMTP_ADDR", "localhost:1025"),
		From: "goticket@goticket.local",
		To:   env("NOTIFY_TO", "agentes@goticket.local"),
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.NewSLAWorker(ticketRepo, hub, notifier, 30*time.Second).Run(ctx)
	}()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api no ar", "addr", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		stop()
		wg.Wait()
		return err
	case <-ctx.Done():
	}

	slog.Info("encerrando...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	wg.Wait()
	return err
}
