package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchAndStoreIconCandidateRejectsHTMLFaviconICO(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><title>not an icon</title></html>"))
	}))
	defer server.Close()

	path, err := fetchAndStoreIconCandidate(context.Background(), server.Client(), server.URL+"/favicon.ico", t.TempDir())
	if err == nil {
		t.Fatalf("expected invalid ico error, got path %q", path)
	}
	if !strings.Contains(err.Error(), "invalid ico") {
		t.Fatalf("expected invalid ico error, got %v", err)
	}
}

func TestFetchAndStoreIconCandidateRejectsInvalidICOContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		_, _ = w.Write([]byte("not-ico"))
	}))
	defer server.Close()

	path, err := fetchAndStoreIconCandidate(context.Background(), server.Client(), server.URL+"/favicon.ico", t.TempDir())
	if err == nil {
		t.Fatalf("expected invalid ico error, got path %q", path)
	}
}

func TestFetchAndStoreIconCandidateRejectsTinyImage(t *testing.T) {
	var payload bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&payload, img); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload.Bytes())
	}))
	defer server.Close()

	path, err := fetchAndStoreIconCandidate(context.Background(), server.Client(), server.URL+"/favicon.png", t.TempDir())
	if err == nil {
		t.Fatalf("expected tiny image error, got path %q", path)
	}
	if !strings.Contains(err.Error(), "too small") {
		t.Fatalf("expected too small error, got %v", err)
	}
}
