package handler

import (
	"bytes"
	"io"
	"net/http/httptest"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/metrics"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler exposes Prometheus metrics at /metrics via promhttp.HandlerFor.
type MetricsHandler struct{}

// NewMetricsHandler creates a new instance.
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

// Handle serves collected metrics via HTTP.
func (h *MetricsHandler) Handle(c fiber.Ctx) error {
	hp := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})

	// Use httptest to capture promhttp output then forward to Fiber response
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	hp.ServeHTTP(rec, req)

	c.Set("Content-Type", rec.Header().Get("Content-Type"))
	bodyBytes, _ := io.ReadAll(io.NopCloser(bytes.NewReader(rec.Body.Bytes())))
	return c.Send(bodyBytes)
}
