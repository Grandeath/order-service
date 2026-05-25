package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Grandeath/order-service/internal/order/domain"
	"github.com/Grandeath/order-service/internal/order/service"
	"github.com/Grandeath/order-service/internal/server"
	"github.com/Grandeath/order-service/internal/utils"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Endpoints() []*server.Endpoint {
	return []*server.Endpoint{
		{Method: http.MethodPost, Path: "/api/v1/orders", Handler: h.create},
		{Method: http.MethodGet, Path: "/api/v1/orders", Handler: h.list},
		{Method: http.MethodGet, Path: "/api/v1/orders/{id}", Handler: h.get},
		{Method: http.MethodPost, Path: "/api/v1/orders/{id}/cancel", Handler: h.cancel},
		{Method: http.MethodPatch, Path: "/api/v1/orders/{id}/status", Handler: h.changeStatus},
		{Method: http.MethodDelete, Path: "/api/v1/orders/{id}", Handler: h.delete},
	}
}

func (h *Handler) Middlewares() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{RequestID, AccessLog, Recovery}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	items := make([]service.ItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, service.ItemInput{
			ProductID:   it.ProductID,
			ProductName: it.ProductName,
			Quantity:    it.Quantity,
			UnitPrice:   it.UnitPrice,
		})
	}

	order, err := h.svc.Create(r.Context(), service.CreateInput{
		IdempotencyKey:  r.Header.Get("Idempotency-Key"),
		CustomerID:      req.CustomerID,
		Currency:        req.Currency,
		Items:           items,
		DeliveryAddress: req.DeliveryAddress,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Location", "/api/v1/orders/"+order.ID)
	utils.WriteJSON(w, http.StatusCreated, toResponse(order))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	customerID := q.Get("customerId")

	orders, err := h.svc.List(r.Context(), customerID, limit, offset)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := ListOrdersResponse{
		Orders: make([]OrderResponse, 0, len(orders)),
		Limit:  limit,
		Offset: offset,
	}
	for _, o := range orders {
		resp.Orders = append(resp.Orders, toResponse(o))
	}
	utils.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, toResponse(order))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req CancelOrderRequest
	// Body is optional for cancel — only decode if present.
	if r.ContentLength != 0 {
		if err := decodeBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}

	order, err := h.svc.Cancel(r.Context(), id, strings.TrimSpace(req.Reason))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, toResponse(order))
}

func (h *Handler) changeStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req ChangeStatusRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	order, err := h.svc.Advance(r.Context(), id, req.Status, strings.TrimSpace(req.Reason))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, toResponse(order))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeBody(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is empty")
		}
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	utils.WriteJSON(w, status, ErrorResponse{Error: msg, Code: code})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrTerminalStatus),
		errors.Is(err, domain.ErrInvalidStatus):
		writeError(w, http.StatusConflict, "invalid_transition", err.Error())
	case errors.Is(err, domain.ErrEmptyItems),
		errors.Is(err, domain.ErrInvalidQuantity),
		errors.Is(err, domain.ErrInvalidUnitPrice),
		errors.Is(err, domain.ErrInvalidCurrency),
		errors.Is(err, domain.ErrInvalidCustomer),
		errors.Is(err, domain.ErrInvalidAddress):
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
