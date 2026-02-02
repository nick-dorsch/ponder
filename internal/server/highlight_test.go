package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ldi/ponder/embed/graph_assets"
)

func TestNodeHighlight(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(graph_assets.Assets)))

	t.Run("Check selectedNodeId and stroke logic in graph.js", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/graph.js", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status OK, got %v", w.Code)
		}

		body, err := io.ReadAll(w.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		content := string(body)

		if !strings.Contains(content, "let selectedNodeId = null;") {
			t.Error("graph.js missing selectedNodeId declaration")
		}

		if !strings.Contains(content, "selectedNodeId = d.id;") {
			t.Error("graph.js missing selectedNodeId assignment in handleNodeClick")
		}

		if !strings.Contains(content, "d.id === selectedNodeId ? '#ffffff' : 'none'") {
			t.Error("graph.js missing stroke highlight logic")
		}

		if !strings.Contains(content, "d.id === selectedNodeId ? 3 : 0") {
			t.Error("graph.js missing stroke-width highlight logic")
		}
	})
}
