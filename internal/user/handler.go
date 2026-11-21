package user

import (
	"database/sql"
	"errors"
	"net/http"

	"goticket/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, agentOnly func(http.Handler) http.Handler) {
	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.Handle("GET /api/me", h.svc.Authenticate(http.HandlerFunc(h.me)))
	mux.Handle("GET /api/users", agentOnly(http.HandlerFunc(h.list)))
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Name, Password string }
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	u, err := h.svc.Register(r.Context(), in.Email, in.Name, in.Password)
	switch {
	case errors.Is(err, ErrValidation):
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrEmailTaken):
		httpx.Error(w, http.StatusConflict, err.Error())
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	default:
		httpx.JSON(w, http.StatusCreated, u)
	}
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	token, u, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		httpx.Error(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	c, _ := FromContext(r.Context())
	u, err := h.svc.Me(r.Context(), c.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, http.StatusUnauthorized, ErrUnauthorized.Error())
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context(), Role(r.URL.Query().Get("role")))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
		return
	}
	httpx.JSON(w, http.StatusOK, users)
}
