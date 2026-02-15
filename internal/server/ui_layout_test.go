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

		if !strings.Contains(content, "width: var(--sidebar-width, 320px);") {
			t.Error("index.html missing CSS variable width for .task-panel")
		}
		if !strings.Contains(content, "class=\"sidebar-resize-handle\"") {
			t.Error("index.html missing visible sidebar resize handle")
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

		if !strings.Contains(content, "const SIDEBAR_MIN_WIDTH = 220;") {
			t.Error("graph.js missing sidebar minimum width constraint")
		}
		if !strings.Contains(content, "const SIDEBAR_MAX_WIDTH_RATIO = 0.5;") {
			t.Error("graph.js missing sidebar maximum width ratio constraint")
		}
		if !strings.Contains(content, "addEventListener('dblclick'") {
			t.Error("graph.js missing double-click handler for sidebar resize reset")
		}
		if !strings.Contains(content, "addEventListener('pointerdown'") {
			t.Error("graph.js missing pointer drag handler for sidebar resize")
		}
	})
}
