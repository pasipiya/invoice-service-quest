// Package httpapi exposes the invoice flow over HTTP.
package httpapi

import (
	"net/http"

	"github.com/pasipiya/invoice-service-quest/internal/invoice"
)

// NewRouter wires the routes. Go 1.22+ pattern routing is used so the service
// carries no third-party web framework.
func NewRouter(svc *invoice.Service) http.Handler {
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /orders/{id}/invoice", h.getInvoice)
	mux.HandleFunc("GET /orders/{id}/summary", h.getSummary)
	return mux
}
