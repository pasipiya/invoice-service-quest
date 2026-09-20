package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/pasipiya/invoice-service-quest/internal/dbtrace"
	"github.com/pasipiya/invoice-service-quest/internal/invoice"
	"github.com/pasipiya/invoice-service-quest/internal/store"
)

type handler struct{ svc *invoice.Service }

func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (h *handler) getInvoice(w http.ResponseWriter, r *http.Request) {
	id, ok := orderID(w, r)
	if !ok {
		return
	}

	// Count the statements this request costs and report it in a response
	// header, so the cost of the flow is observable with curl alone.
	ctx, counter := dbtrace.WithCounter(r.Context())

	inv, err := h.svc.Build(ctx, id)
	if err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("X-Query-Count", strconv.Itoa(counter.Count()))
	writeJSON(w, inv)
}

func (h *handler) getSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := orderID(w, r)
	if !ok {
		return
	}

	ctx, counter := dbtrace.WithCounter(r.Context())

	sum, err := h.svc.Summarise(ctx, id)
	if err != nil {
		writeErr(w, err)
		return
	}

	w.Header().Set("X-Query-Count", strconv.Itoa(counter.Count()))
	writeJSON(w, sum)
}

// orderID parses the path parameter, writing a 400 and reporting false when it
// is not a positive integer.
func orderID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "order id must be a positive integer", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	slog.Error("invoice request failed", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}
