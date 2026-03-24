package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler := NewSystemHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	engine.GET("/healthz", handler.Healthz)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
