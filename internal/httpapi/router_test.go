package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kmainstay/internal/httpapi"
)

func TestSPA_WhenRequested_ServesBrowserRoutesWithoutShadowingAPI(t *testing.T) {
	assets := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<main>application</main>")
	})
	handler := httpapi.New(httpapi.Dependencies{Assets: assets})
	for _, path := range []string{"/", "/conversation/con_1"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "application") {
			t.Fatalf("GET %s = %d %q", path, recorder.Code, recorder.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "application") {
		t.Fatalf("API fallback = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestHealth_WhenRequested_ReturnsOK(t *testing.T) {
	recorder := httptest.NewRecorder()
	httpapi.New(httpapi.Dependencies{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("body = %q, err = %v", recorder.Body.String(), err)
	}
}
