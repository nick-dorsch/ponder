package server

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUILayout(t *testing.T) {
	mux := testMux()

	t.Run("Check sidebar width in index.html", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		body, _ := io.ReadAll(w.Body)
		content := string(body)

		if !strings.Contains(content, "width: 24vw;") {
			t.Error("index.html missing width: 24vw; for .task-panel")
		}
		if !strings.Contains(content, "direction: rtl;") {
			t.Error("index.html missing direction: rtl; for .task-list")
		}
	})

	t.Run("Check sidebar width calculation in graph.js", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/graph.js", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		body, _ := io.ReadAll(w.Body)
		content := string(body)

		if !strings.Contains(content, "SIDEBAR_WIDTH = WIDTH * 0.24") {
			t.Error("graph.js missing SIDEBAR_WIDTH calculation with 0.24 factor")
		}
	})
}
