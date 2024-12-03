package response

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Response represents standard API response
type Response struct {
	Code      int         `json:"code"`            // HTTP status code
	Message   string      `json:"message"`         // Response message
	Data      interface{} `json:"data,omitempty"`  // Response data
	Error     string      `json:"error,omitempty"` // Error message if any
	RequestID string      `json:"request_id"`      // Request ID for tracking
	Timestamp time.Time   `json:"timestamp"`       // Response timestamp
}

// Handler provides methods for standard API responses
type Handler struct {
	ctx    *gin.Context
	logger *zap.Logger
}

// New creates new response handler
func New(c *gin.Context, logger *zap.Logger) *Handler {
	return &Handler{
		ctx:    c,
		logger: logger,
	}
}

// Success sends success response
func (h *Handler) Success(data interface{}) {
	h.ctx.JSON(http.StatusOK, Response{
		Code:      http.StatusOK,
		Message:   "success",
		Data:      data,
		RequestID: h.ctx.GetString("request_id"),
		Timestamp: time.Now(),
	})
}

// Created sends created response
func (h *Handler) Created(data interface{}) {
	h.ctx.JSON(http.StatusCreated, Response{
		Code:      http.StatusCreated,
		Message:   "created",
		Data:      data,
		RequestID: h.ctx.GetString("request_id"),
		Timestamp: time.Now(),
	})
}

// NoContent sends no content response
func (h *Handler) NoContent() {
	h.ctx.JSON(http.StatusNoContent, nil)
}

// Error sends an error response
func (h *Handler) Error(status int, err error) {
	h.ctx.JSON(status, Response{
		Code:      status,
		Message:   "error",
		Error:     err.Error(),
		RequestID: h.ctx.GetString("request_id"),
		Timestamp: time.Now(),
	})
}

// BadRequest sends bad request error response
func (h *Handler) BadRequest(err error) {
	h.Error(http.StatusBadRequest, err)
}

// NotFound sends not found error response
func (h *Handler) NotFound(err error) {
	h.Error(http.StatusNotFound, err)
}

// ValidationError sends validation error response
func (h *Handler) ValidationError(err error) {
	h.Error(http.StatusUnprocessableEntity, err)
}

// InternalError sends an internal server error response
func (h *Handler) InternalError(err error) {
	h.Error(http.StatusInternalServerError, err)
}

// Custom sends custom response
func (h *Handler) Custom(status int, resp interface{}) {
	h.ctx.JSON(status, resp)
}

// File sends file response
func (h *Handler) File(filepath string) {
	h.ctx.File(filepath)
}

// StreamWriter defines stream writer interface
type StreamWriter interface {
	Write([]byte) (int, error)
	Flush() error
}

// StreamOptions defines stream options
type StreamOptions struct {
	ContentType string
	Headers     map[string]string
	BufferSize  int
}

// DefaultStreamOptions returns default stream options
func DefaultStreamOptions() StreamOptions {
	return StreamOptions{
		ContentType: "text/plain",
		BufferSize:  4096,
	}
}

// Stream handles streaming response
func (h *Handler) Stream(reader io.Reader, opts ...StreamOptions) {
	var options StreamOptions
	if len(opts) > 0 {
		options = opts[0]
	} else {
		options = DefaultStreamOptions()
	}

	h.ctx.Header("Content-Type", options.ContentType)
	h.ctx.Header("Transfer-Encoding", "chunked")
	h.ctx.Header("X-Content-Type-Options", "nosniff")

	for k, v := range options.Headers {
		h.ctx.Header(k, v)
	}

	writer := h.ctx.Writer
	bufWriter := bufio.NewWriterSize(writer, options.BufferSize)

	h.ctx.Stream(func(w io.Writer) bool {
		buf := make([]byte, options.BufferSize)
		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				h.logger.Error("stream read error",
					zap.Error(err),
					zap.String("request_id", h.ctx.GetString("request_id")))
			}
			return false
		}

		if n > 0 {
			if _, err := bufWriter.Write(buf[:n]); err != nil {
				h.logger.Error("stream write error",
					zap.Error(err),
					zap.String("request_id", h.ctx.GetString("request_id")))
				return false
			}
			bufWriter.Flush()
		}

		return true
	})
}

// StreamJSON streams JSON data
func (h *Handler) StreamJSON(data <-chan interface{}) {
	opts := StreamOptions{
		ContentType: "application/json",
		BufferSize:  4096,
	}

	encoder := json.NewEncoder(h.ctx.Writer)
	h.ctx.Header("Content-Type", opts.ContentType)
	h.ctx.Header("Transfer-Encoding", "chunked")

	h.ctx.Stream(func(w io.Writer) bool {
		select {
		case item, ok := <-data:
			if !ok {
				return false
			}
			if err := encoder.Encode(item); err != nil {
				h.logger.Error("json encode error",
					zap.Error(err),
					zap.String("request_id", h.ctx.GetString("request_id")))
				return false
			}
			h.ctx.Writer.Flush()
			return true
		case <-h.ctx.Request.Context().Done():
			return false
		}
	})
}

// SSEvent defines SSE event structure
type SSEvent struct {
	Event string
	Data  string
}

// StreamSSE sends Server-Sent Events
func (h *Handler) StreamSSE(events <-chan SSEvent) {
	opts := StreamOptions{
		ContentType: "text/event-stream",
		Headers: map[string]string{
			"Cache-Control": "no-cache",
			"Connection":    "keep-alive",
		},
	}

	h.ctx.Header("Content-Type", opts.ContentType)
	for k, v := range opts.Headers {
		h.ctx.Header(k, v)
	}

	h.ctx.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-events:
			if !ok {
				return false
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Event, event.Data); err != nil {
				h.logger.Error("sse write error",
					zap.Error(err),
					zap.String("request_id", h.ctx.GetString("request_id")))
				return false
			}
			h.ctx.Writer.Flush()
			return true
		case <-h.ctx.Request.Context().Done():
			return false
		}
	})
}
