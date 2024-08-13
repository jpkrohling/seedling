package config

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Create is an HTTP handler that accepts an OTel Collector configuration
type Create struct {
	tracer     trace.Tracer
	processors []Processor
}

type Response struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	ID      uuid.UUID `json:"id"`
}

type Processor interface {
	Process(context.Context, io.ReadCloser) error
}

// NewHandler returns a new Handler
func NewHandler(opts ...func(*Create)) *Create {
	h := &Create{}

	for _, opt := range opts {
		opt(h)
	}

	if h.tracer == nil {
		h.tracer = otel.Tracer("config")
	}

	return h
}

// WithTracer sets the tracer on the Handler
func WithTracer(tracer trace.Tracer) func(*Create) {
	return func(h *Create) {
		h.tracer = tracer
	}
}

// WithProcessors sets the processors on the Handler
func WithProcessors(processors ...Processor) func(*Create) {
	return func(h *Create) {
		h.processors = processors
	}
}

// ServeHTTP implements http.Handler
func (h *Create) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "CreateConfig")
	defer span.End()

	id := uuid.New()
	span.SetAttributes(attribute.Stringer("id", id))

	// verify that we have a POST request
	if r.Method != "POST" {
		span.SetAttributes(attribute.String("method", r.Method))
		span.SetStatus(codes.Error, "invalid request method")
		writeResponse(span, w, http.StatusMethodNotAllowed, Response{
			Success: false,
			Message: "Invalid request method",
		})

		return
	}

	// verify we have a YAML payload
	if r.Header.Get("Content-Type") != "application/yaml" {
		span.SetAttributes(attribute.String("content-type", r.Header.Get("Content-Type")))
		span.SetStatus(codes.Error, "invalid request content type")
		writeResponse(span, w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Invalid content type",
		})
		return
	}

	// process the request
	for _, p := range h.processors {
		if err := p.Process(ctx, r.Body); err != nil {
			span.SetStatus(codes.Error, err.Error())
			writeResponse(span, w, http.StatusInternalServerError, Response{
				Success: false,
				Message: "Failed to process configuration",
			})
			return
		}
	}

	span.SetAttributes(attribute.Bool("success", true))

	// send the response
	writeResponse(span, w, http.StatusOK, Response{
		Success: true,
		Message: "Configuration received",
		ID:      id,
	})
}

func writeResponse(span trace.Span, w http.ResponseWriter, statusCode int, resp Response) {
	respBody, err := json.Marshal(resp)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal response")
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(respBody); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write response")
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}
