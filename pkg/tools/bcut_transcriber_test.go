package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
)

func TestUploadPartWithRetry_RetriesTransientStatus(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := attempts.Add(1)
		if current < 3 {
			w.WriteHeader(http.StatusGatewayTimeout)
			_, _ = w.Write([]byte("gateway timeout"))
			return
		}
		w.Header().Set("Etag", `"etag-ok"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tool := &BcutTranscriberTool{
		client: server.Client(),
		logger: zap.NewNop(),
	}

	etag, err := tool.uploadPartWithRetry(context.Background(), server.URL, []byte("hello"), 0, 1)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if etag != "etag-ok" {
		t.Fatalf("expected etag-ok, got %q", etag)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestUploadPartWithRetry_DoesNotRetryNonRetryableStatus(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	tool := &BcutTranscriberTool{
		client: server.Client(),
		logger: zap.NewNop(),
	}

	_, err := tool.uploadPartWithRetry(context.Background(), server.URL, []byte("hello"), 0, 1)
	if err == nil {
		t.Fatal("expected non-retryable upload error")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected 1 attempt for non-retryable status, got %d", got)
	}
	if want := "status 403"; !containsString(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got %v", want, err)
	}
}

func containsString(value, needle string) bool {
	return fmt.Sprintf("%s", value) != "" && len(needle) > 0 && (len(value) >= len(needle)) && (func() bool {
		return stringContains(value, needle)
	})()
}

func stringContains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
