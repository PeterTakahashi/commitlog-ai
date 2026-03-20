package server

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/anthropics/commitlog-ai/internal/linker"
	"github.com/anthropics/commitlog-ai/internal/model"
)

// Server serves the React UI and API endpoints.
type Server struct {
	ProjectDir string
	Port       int
	StaticFS   fs.FS        // embedded or filesystem-based
	OnReady    func(port int) // called once the server is listening
	mu         sync.RWMutex
	timeline   *model.LinkedTimeline
	sessions   map[string]*model.Session
	gitClient  *linker.GitClient
	avatarCache sync.Map // email -> avatar URL
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
	timelinePath := filepath.Join(s.ProjectDir, ".commitlog-ai", "timeline.json")
	data, err := os.ReadFile(timelinePath)
	if err != nil {
		return fmt.Errorf("no timeline found. Run 'commitlog-ai parse && commitlog-ai link' first: %w", err)
	}

	var timeline model.LinkedTimeline
	if err := json.Unmarshal(data, &timeline); err != nil {
		return fmt.Errorf("parsing timeline.json: %w", err)
	}

	sessions := make(map[string]*model.Session)
	for i, entry := range timeline.Entries {
		if entry.Session != nil {
			sessions[entry.Session.ID] = timeline.Entries[i].Session
		}
	}

	s.mu.Lock()
	s.timeline = &timeline
	s.sessions = sessions
	s.mu.Unlock()

	// Setup git client (idempotent)
	if s.gitClient == nil {
		s.gitClient = linker.NewGitClient(s.ProjectDir)
	}

	return nil
}

// ReloadData re-reads timeline.json from disk and updates in-memory data.
func (s *Server) ReloadData() error {
	return s.loadData()
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
	mux.HandleFunc("/api/sessions-commits/", s.handleSessionCommits)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/avatar", s.handleAvatar)

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
	fmt.Printf("commitlog-ai server running at http://%s\n", addr)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

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

	// Support message range slicing for segmented sessions
	q := r.URL.Query()
	startStr := q.Get("start")
	endStr := q.Get("end")

	if startStr != "" && endStr != "" {
		start, _ := strconv.Atoi(startStr)
		end, _ := strconv.Atoi(endStr)
		if start >= 0 && end > start && end <= len(session.Messages) {
			sliced := *session
			sliced.Messages = session.Messages[start:end]
			writeJSON(w, sliced)
			return
		}
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

func (s *Server) handleSessionCommits(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// /api/sessions-commits/{id} → all commits linked to this session with diffs
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions-commits/")
	if id == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	type commitWithDiff struct {
		model.GitCommit
		Diff string `json:"diff"`
	}

	var results []commitWithDiff
	seen := make(map[string]bool)
	for _, entry := range s.timeline.Entries {
		if entry.Session != nil && entry.Session.ID == id && entry.Commit != nil {
			if seen[entry.Commit.Hash] {
				continue
			}
			seen[entry.Commit.Hash] = true
			diff, _ := s.gitClient.GetDiff(entry.Commit.Hash)
			results = append(results, commitWithDiff{
				GitCommit: *entry.Commit,
				Diff:      diff,
			})
		}
	}

	writeJSON(w, results)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]any{
		"total_entries":  len(s.timeline.Entries),
		"total_sessions": len(s.sessions),
	}

	// Count by agent + token usage
	agentCounts := make(map[string]int)
	totalMessages := 0
	type tokenStat struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	}
	tokenByAgent := make(map[string]*tokenStat)
	for _, session := range s.sessions {
		agent := session.Agent.Tool
		agentCounts[agent]++
		totalMessages += len(session.Messages)
		ts, ok := tokenByAgent[agent]
		if !ok {
			ts = &tokenStat{}
			tokenByAgent[agent] = ts
		}
		for _, msg := range session.Messages {
			if msg.Usage != nil {
				ts.InputTokens += msg.Usage.InputTokens
				ts.OutputTokens += msg.Usage.OutputTokens
				ts.CacheCreationInputTokens += msg.Usage.CacheCreationInputTokens
				ts.CacheReadInputTokens += msg.Usage.CacheReadInputTokens
			}
		}
	}
	stats["by_agent"] = agentCounts
	stats["total_messages"] = totalMessages
	stats["token_by_agent"] = tokenByAgent

	// Count linked vs unlinked + diff stats per agent
	linked := 0
	commitOnly := 0
	sessionOnly := 0
	type diffStat struct {
		Additions    int `json:"additions"`
		Deletions    int `json:"deletions"`
		FilesChanged int `json:"files_changed"`
		Commits      int `json:"commits"`
	}
	diffByAgent := make(map[string]*diffStat)
	seenCommits := make(map[string]string) // hash -> agent (avoid double-counting)
	for _, e := range s.timeline.Entries {
		if e.Commit != nil && e.Session != nil {
			linked++
			agent := e.Session.Agent.Tool
			if _, ok := seenCommits[e.Commit.Hash]; !ok {
				seenCommits[e.Commit.Hash] = agent
				ds, ok := diffByAgent[agent]
				if !ok {
					ds = &diffStat{}
					diffByAgent[agent] = ds
				}
				ds.Additions += e.Commit.Additions
				ds.Deletions += e.Commit.Deletions
				ds.FilesChanged += e.Commit.FilesChanged
				ds.Commits++
			}
		} else if e.Commit != nil {
			commitOnly++
		} else {
			sessionOnly++
		}
	}
	stats["linked"] = linked
	stats["commit_only"] = commitOnly
	stats["session_only"] = sessionOnly
	stats["diff_by_agent"] = diffByAgent

	writeJSON(w, stats)
}

func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	if email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	// Try GitHub avatar first by searching commit email via GitHub API
	// Fall back to Gravatar which works universally
	avatarURL := gravatarURL(email, 80)

	// Try to resolve GitHub username from email (check cache first)
	if cached, ok := s.avatarCache.Load(email); ok {
		avatarURL = cached.(string)
	} else {
		// Try GitHub search API (unauthenticated, rate-limited)
		ghURL := resolveGitHubAvatar(email)
		if ghURL != "" {
			avatarURL = ghURL
		}
		s.avatarCache.Store(email, avatarURL)
	}

	writeJSON(w, map[string]string{"avatar_url": avatarURL})
}

func gravatarURL(email string, size int) string {
	hash := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return fmt.Sprintf("https://www.gravatar.com/avatar/%x?s=%d&d=identicon", hash, size)
}

func resolveGitHubAvatar(email string) string {
	// Use GitHub's search API to find user by email
	url := fmt.Sprintf("https://api.github.com/search/users?q=%s+in:email", email)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	if len(result.Items) > 0 {
		return result.Items[0].AvatarURL
	}
	return ""
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(data)
}
