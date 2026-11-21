package ticket

import (
	"errors"
	"net/http"
	"strconv"

	"goticket/internal/httpx"
	"goticket/internal/user"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register monta as rotas protegidas por auth (Go 1.22+ method/path patterns).
func (h *Handler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	route := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, auth(fn)) }
	route("GET /api/categories", h.categories)
	route("GET /api/tickets", h.list)
	route("POST /api/tickets", h.create)
	route("GET /api/tickets/{id}", h.get)
	route("POST /api/tickets/{id}/transition", h.transition)
	route("POST /api/tickets/{id}/assign", h.assign)
	route("POST /api/tickets/{id}/comments", h.comment)
}

func actor(r *http.Request) user.Claims { c, _ := user.FromContext(r.Context()); return c }

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInvalidTransition):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrValidation):
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "erro interno")
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	ts, err := h.svc.List(r.Context(), actor(r), ListQuery{
		Status: Status(qs.Get("status")), Search: qs.Get("q"), Assignee: qs.Get("assignee"),
	})
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ts)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	t, err := h.svc.Create(r.Context(), actor(r), in)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return
	}
	d, err := h.svc.Get(r.Context(), actor(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	var in struct {
		To Status `json:"to"`
	}
	if !ok || httpx.Decode(w, r, &in) != nil {
		httpx.Error(w, http.StatusBadRequest, "requisição inválida")
		return
	}
	t, err := h.svc.Transition(r.Context(), actor(r), id, in.To)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	var in struct {
		AssigneeID int64 `json:"assigneeId"`
	}
	if !ok || httpx.Decode(w, r, &in) != nil || in.AssigneeID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "requisição inválida")
		return
	}
	t, err := h.svc.Assign(r.Context(), actor(r), id, in.AssigneeID)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) comment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	var in struct {
		Body     string `json:"body"`
		Internal bool   `json:"internal"`
	}
	if !ok || httpx.Decode(w, r, &in) != nil {
		httpx.Error(w, http.StatusBadRequest, "requisição inválida")
		return
	}
	c, err := h.svc.AddComment(r.Context(), actor(r), id, in.Body, in.Internal)
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.Categories(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cs)
}
