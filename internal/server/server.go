package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anthropics/aitrace/internal/linker"
	"github.com/anthropics/aitrace/internal/model"
)

// Server serves the React UI and API endpoints.
type Server struct {
	ProjectDir string
	Port       int
	StaticFS   fs.FS        // embedded or filesystem-based
	OnReady    func(port int) // called once the server is listening
	timeline   *model.LinkedTimeline
	sessions   map[string]*model.Session
	gitClient  *linker.GitClient
}

// New creates a new server instance.
func New(projectDir string, port int, staticFS fs.FS) *Server {
	return &Server{
		ProjectDir: projectDir,
		Port:       port,
		StaticFS:   staticFS,
		sessions:   make(map[string]*model.Session),
	}
}

func (s *Server) loadData() error {
	// Load timeline
	timelinePath := filepath.Join(s.ProjectDir, ".aitrace", "timeline.json")
	data, err := os.ReadFile(timelinePath)
	if err != nil {
		return fmt.Errorf("no timeline found. Run 'aitrace parse && aitrace link' first: %w", err)
	}

	var timeline model.LinkedTimeline
	if err := json.Unmarshal(data, &timeline); err != nil {
		return fmt.Errorf("parsing timeline.json: %w", err)
	}
	s.timeline = &timeline

	// Index sessions by ID
	for i, entry := range timeline.Entries {
		if entry.Session != nil {
			s.sessions[entry.Session.ID] = timeline.Entries[i].Session
		}
	}

	// Setup git client
	s.gitClient = linker.NewGitClient(s.ProjectDir)

	return nil
}

// Start loads data and starts the HTTP server.
func (s *Server) Start() error {
	if err := s.loadData(); err != nil {
		return err
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/timeline", s.handleTimeline)
	mux.HandleFunc("/api/sessions/", s.handleSession)
	mux.HandleFunc("/api/commits/", s.handleCommitDiff)
	mux.HandleFunc("/api/stats", s.handleStats)

	// Static files (React SPA)
	if s.StaticFS != nil {
		fileServer := http.FileServer(http.FS(s.StaticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the file directly
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			}
			// Remove leading slash for fs.Open
			fsPath := strings.TrimPrefix(path, "/")
			if _, err := fs.Stat(s.StaticFS, fsPath); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
			// SPA fallback: serve index.html for client-side routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}

	// Try the requested port, then find an available one
	requestedPort := s.Port
	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", requestedPort))
	if err != nil {
		// Port is busy, find a free one
		ln, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		actualPort := ln.Addr().(*net.TCPAddr).Port
		fmt.Printf("Port %d is in use, using port %d instead\n", requestedPort, actualPort)
		s.Port = actualPort
	}

	addr := fmt.Sprintf("localhost:%d", s.Port)
	fmt.Printf("aitrace server running at http://%s\n", addr)
	fmt.Printf("  %d timeline entries, %d sessions\n", len(s.timeline.Entries), len(s.sessions))

	if s.OnReady != nil {
		go s.OnReady(s.Port)
	}

	return http.Serve(ln, mux)
}

// PaginatedTimeline is the paginated API response for /api/timeline.
type PaginatedTimeline struct {
	Entries  []model.TimelineEntry `json:"entries"`
	GitRepo  string                `json:"git_repo"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	HasMore  bool                  `json:"has_more"`
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	agentFilter := q.Get("agent")
	search := strings.ToLower(q.Get("q"))

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	// Filter
	var filtered []model.TimelineEntry
	for _, e := range s.timeline.Entries {
		if agentFilter != "" {
			if e.Session == nil || e.Session.Agent.Tool != agentFilter {
				continue
			}
		}
		if search != "" && !entryMatchesSearch(e, search) {
			continue
		}
		filtered = append(filtered, e)
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	pageEntries := make([]model.TimelineEntry, 0, end-start)
	for _, e := range filtered[start:end] {
		entry := e
		if entry.Session != nil {
			summarized := *entry.Session
			summarized.Messages = nil
			entry.Session = &summarized
		}
		pageEntries = append(pageEntries, entry)
	}

	writeJSON(w, PaginatedTimeline{
		Entries:  pageEntries,
		GitRepo:  s.timeline.GitRepo,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	})
}

func entryMatchesSearch(e model.TimelineEntry, query string) bool {
	if e.Commit != nil {
		if strings.Contains(strings.ToLower(e.Commit.Message), query) {
			return true
		}
		if strings.Contains(e.Commit.Hash, query) {
			return true
		}
	}
	if e.Session != nil {
		if strings.Contains(strings.ToLower(e.Session.Project), query) {
			return true
		}
		if strings.Contains(strings.ToLower(e.Session.Agent.Model), query) {
			return true
		}
	}
	return false
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path: /api/sessions/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if id == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	session, ok := s.sessions[id]
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	writeJSON(w, session)
}

func (s *Server) handleCommitDiff(w http.ResponseWriter, r *http.Request) {
	// Extract hash from path: /api/commits/{hash}/diff
	path := strings.TrimPrefix(r.URL.Path, "/api/commits/")
	hash := strings.TrimSuffix(path, "/diff")
	if hash == "" {
		http.Error(w, "commit hash required", http.StatusBadRequest)
		return
	}

	diff, err := s.gitClient.GetDiff(hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"diff": diff})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]any{
		"total_entries":  len(s.timeline.Entries),
		"total_sessions": len(s.sessions),
	}

	// Count by agent
	agentCounts := make(map[string]int)
	totalMessages := 0
	for _, session := range s.sessions {
		agentCounts[session.Agent.Tool]++
		totalMessages += len(session.Messages)
	}
	stats["by_agent"] = agentCounts
	stats["total_messages"] = totalMessages

	// Count linked vs unlinked
	linked := 0
	commitOnly := 0
	sessionOnly := 0
	for _, e := range s.timeline.Entries {
		if e.Commit != nil && e.Session != nil {
			linked++
		} else if e.Commit != nil {
			commitOnly++
		} else {
			sessionOnly++
		}
	}
	stats["linked"] = linked
	stats["commit_only"] = commitOnly
	stats["session_only"] = sessionOnly

	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(data)
}
