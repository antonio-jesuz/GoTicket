// Package worker agrupa rotinas assíncronas: checagem de SLA e notificações.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"time"

	"goticket/internal/realtime"
	"goticket/internal/ticket"
)

type Notifier interface {
	Notify(ctx context.Context, subject, body string) error
}

type SLAWorker struct {
	repo     ticket.Repository
	pub      ticket.Publisher
	notifier Notifier
	interval time.Duration
}

func NewSLAWorker(repo ticket.Repository, pub ticket.Publisher, n Notifier, interval time.Duration) *SLAWorker {
	return &SLAWorker{repo: repo, pub: pub, notifier: n, interval: interval}
}

// Run bloqueia até o ctx ser cancelado (graceful shutdown).
func (w *SLAWorker) Run(ctx context.Context) {
	tick := time.NewTicker(w.interval)
	defer tick.Stop()
	slog.Info("sla worker iniciado", "interval", w.interval)
	for {
		w.check(ctx)
		select {
		case <-ctx.Done():
			slog.Info("sla worker encerrado")
			return
		case <-tick.C:
		}
	}
}

func (w *SLAWorker) check(ctx context.Context) {
	breached, err := w.repo.MarkSLABreached(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("sla: falha ao marcar violações", "err", err)
		}
		return
	}
	for _, t := range breached {
		slog.Warn("SLA violado", "ticket", t.Number, "priority", t.Priority)
		w.pub.Publish(realtime.Event{Type: "sla.breached", TicketID: t.ID, OwnerID: t.RequesterID, Data: t})
		subject := fmt.Sprintf("[SLA violado] %s (%s)", t.Number, t.Priority)
		body := fmt.Sprintf("O ticket %s — %q — passou do prazo de SLA (%s).",
			t.Number, t.Title, t.SLADueAt.Format(time.RFC1123))
		if err := w.notifier.Notify(ctx, subject, body); err != nil {
			slog.Error("sla: falha ao notificar", "ticket", t.Number, "err", err)
		}
	}
}

// SMTPNotifier envia e-mails; em dev aponta para o Mailpit (localhost:1025, sem auth).
type SMTPNotifier struct{ Addr, From, To string }

func (n SMTPNotifier) Notify(_ context.Context, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		n.From, n.To, subject, body)
	return smtp.SendMail(n.Addr, nil, n.From, []string{n.To}, []byte(msg))
}
