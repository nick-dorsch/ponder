package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ldi/ponder/embed/graph_assets"
)

func TestPlaceholderFix(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(graph_assets.Assets)))

	t.Run("Check task-list-placeholder class in index.html", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
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

		if !strings.Contains(content, "class=\"task-list-placeholder\"") {
			t.Error("index.html missing class=\"task-list-placeholder\"")
		}

		if !strings.Contains(content, "Loading tasks...</div>") {
			t.Error("index.html missing Loading tasks... placeholder text")
		}
	})

	t.Run("Check placeholder removal logic in graph.js", func(t *testing.T) {
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

		if !strings.Contains(content, "taskListDiv.querySelector(':scope > .task-list-placeholder')") {
			t.Error("graph.js missing specific placeholder removal logic with :scope and class")
		}

		if strings.Contains(content, "taskListDiv.querySelector('div:not(.feature-group)')") {
			t.Error("graph.js still contains broad placeholder removal logic")
		}
	})
}
