// Package realtime faz broadcast de eventos de ticket via Server-Sent Events.
// SSE foi escolhido sobre WebSocket por ser unidirecional, stdlib-only e reconectar sozinho (ADR-003).
package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"goticket/internal/user"
)

type Event struct {
	Type     string `json:"type"` // ticket.created | ticket.updated | comment.added | sla.breached
	TicketID int64  `json:"ticketId"`
	OwnerID  int64  `json:"-"` // solicitante: clientes só recebem eventos dos próprios tickets
	Internal bool   `json:"-"` // eventos internos (nota privada) não vão para clientes
	Data     any    `json:"data,omitempty"`
}

type client struct {
	ch     chan Event
	userID int64
	agent  bool
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewHub() *Hub { return &Hub{clients: make(map[*client]struct{})} }

// Publish nunca bloqueia: cliente lento perde o evento (a UI recarrega ao reconectar).
func (h *Hub) Publish(e Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.agent && (c.userID != e.OwnerID || e.Internal) {
			continue
		}
		select {
		case c.ch <- e:
		default:
		}
	}
}

func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	claims, ok := user.FromContext(r.Context())
	flusher, canFlush := w.(http.Flusher)
	if !ok || !canFlush {
		http.Error(w, "streaming indisponível", http.StatusInternalServerError)
		return
	}
	c := &client{ch: make(chan Event, 16), userID: claims.UserID, agent: claims.IsAgent()}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case e := <-c.ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
			flusher.Flush()
		}
	}
}
