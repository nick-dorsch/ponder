package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nick-dorsch/ponder/embed/graph_assets"
	"github.com/nick-dorsch/ponder/internal/db"
)

type Server struct {
	db     *db.DB
	server *http.Server
}

func NewServer(database *db.DB) *Server {
	return &Server{db: database}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/features", s.handleFeatures)
	mux.HandleFunc("/api/graph", s.handleGraph)

	// Static files
	mux.Handle("/", http.FileServer(http.FS(graph_assets.Assets)))

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.db.ListTasks(r.Context(), nil, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request) {
	features, err := s.db.ListFeatures(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(features)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	graphJSON, err := s.db.GetGraphJSON(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(graphJSON))
}
