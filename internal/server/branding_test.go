package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ldi/ponder/embed/graph_assets"
)

func TestBranding(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(graph_assets.Assets)))

	t.Run("Check title and header in index.html", func(t *testing.T) {
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

		if !strings.Contains(content, "<title>Ponder</title>") {
			t.Error("index.html missing <title>Ponder</title>")
		}

		if !strings.Contains(content, "P<span class=\"ponder-o\"></span>NDER") {
			t.Error("index.html missing PONDER header with glowing O")
		}

		if strings.Contains(content, "TaskTree") {
			// Some mentions might remain in comments or variable names, but the main ones should be gone
			// Actually, let's be more specific.
			if strings.Contains(content, "<title>TaskTree</title>") || strings.Contains(content, "div class=\"panel-title\">TaskTree</div>") {
				t.Error("index.html still contains TaskTree in title or header")
			}
		}
	})
}
