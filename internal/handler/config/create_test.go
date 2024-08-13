package config

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler()
	assert.NotNil(t, h)
	assert.NotNil(t, h.tracer)
}

func TestWithProcessors(t *testing.T) {
	h := NewHandler(WithProcessors(&mockProcessor{}))
	assert.NotNil(t, h)
	assert.Len(t, h.processors, 1)
}

func TestCreateInvalidMethod(t *testing.T) {
	// prepare
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	// test
	h.ServeHTTP(res, req)

	// verify
	assert.Equal(t, http.StatusMethodNotAllowed, res.Code)
}

func TestCreateInvalidContentType(t *testing.T) {
	// prepare
	h := NewHandler()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	res := httptest.NewRecorder()

	// test
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(res, req)

	// verify
	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestCreate(t *testing.T) {
	// prepare
	m := &mockProcessor{}
	h := NewHandler(WithProcessors(m))
	body := `{"key":"value"}`
	req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(strings.NewReader(body)))
	res := httptest.NewRecorder()

	// test
	req.Header.Set("Content-Type", "application/yaml")
	h.ServeHTTP(res, req)

	// verify
	assert.Equal(t, http.StatusOK, res.Code)
	assert.True(t, m.called)

	var resp Response
	err := json.Unmarshal(res.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, "Configuration received", resp.Message)
}

func TestFailedToProcess(t *testing.T) {
	// prepare
	m := &mockProcessor{
		err: assert.AnError,
	}
	h := NewHandler(WithProcessors(m))
	body := `{"key":"value"}`
	req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(strings.NewReader(body)))
	res := httptest.NewRecorder()

	// test
	req.Header.Set("Content-Type", "application/yaml")
	h.ServeHTTP(res, req)

	// verify
	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.True(t, m.called)

	var resp Response
	err := json.Unmarshal(res.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.Equal(t, "Failed to process configuration", resp.Message)
}

type mockProcessor struct {
	called bool
	err    error
}

func (m *mockProcessor) Process(context.Context, io.ReadCloser) error {
	m.called = true
	return m.err
}
