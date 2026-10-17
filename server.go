package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"BlogAggregator/internal/database"
)

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJSON(w, code, map[string]string{"error": msg})
}

func (s *state) authMiddleware(next func(http.ResponseWriter, *http.Request, database.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		var userName string
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && (strings.EqualFold(parts[0], "Bearer") || strings.EqualFold(parts[0], "ApiKey")) {
				userName = strings.TrimSpace(parts[1])
			} else {
				userName = strings.TrimSpace(authHeader)
			}
		} else if userHeader := r.Header.Get("X-User-Name"); userHeader != "" {
			userName = strings.TrimSpace(userHeader)
		}

		if userName == "" {
			respondWithError(w, http.StatusUnauthorized, "Missing Authorization header (e.g. 'Bearer <username>' or 'X-User-Name')")
			return
		}

		user, err := s.db.GetUser(r.Context(), userName)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("User %q not found", userName))
			return
		}

		next(w, r, user)
	}
}

func (s *state) handleHealthz(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "gator-api",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *state) handleAPIFeeds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	feeds, err := s.db.GetFeeds(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve feeds")
		return
	}

	type FeedResponse struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		URL       string    `json:"url"`
		UserName  string    `json:"user_name"`
		CreatedAt time.Time `json:"created_at"`
	}

	res := make([]FeedResponse, len(feeds))
	for i, f := range feeds {
		res[i] = FeedResponse{
			ID:        f.ID,
			Name:      f.Name,
			URL:       f.Url,
			UserName:  f.UserName,
			CreatedAt: f.CreatedAt,
		}
	}

	respondWithJSON(w, http.StatusOK, res)
}

func (s *state) handleAPIPosts(w http.ResponseWriter, r *http.Request, user database.User) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	limit := int32(10)
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = int32(val)
		}
	}

	page := int32(1)
	if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = int32(val)
		}
	}
	offset := (page - 1) * limit

	posts, err := s.db.GetPostsForUserPaged(r.Context(), database.GetPostsForUserPagedParams{
		UserID: user.ID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to load posts")
		return
	}

	type PostResponse struct {
		ID          uuid.UUID  `json:"id"`
		Title       string     `json:"title"`
		URL         string     `json:"url"`
		Description *string    `json:"description,omitempty"`
		PublishedAt *time.Time `json:"published_at,omitempty"`
		FeedID      uuid.UUID  `json:"feed_id"`
	}

	res := make([]PostResponse, len(posts))
	for i, p := range posts {
		var desc *string
		if p.Description.Valid {
			desc = &p.Description.String
		}
		var pub *time.Time
		if p.PublishedAt.Valid {
			pub = &p.PublishedAt.Time
		}
		res[i] = PostResponse{
			ID:          p.ID,
			Title:       p.Title,
			URL:         p.Url,
			Description: desc,
			PublishedAt: pub,
			FeedID:      p.FeedID,
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"page":  page,
		"limit": limit,
		"count": len(res),
		"posts": res,
	})
}

func (s *state) handleAPISearch(w http.ResponseWriter, r *http.Request, user database.User) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		respondWithError(w, http.StatusBadRequest, "Missing 'q' query parameter")
		return
	}

	limit := int32(10)
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = int32(val)
		}
	}

	posts, err := s.db.SearchPostsForUser(r.Context(), database.SearchPostsForUserParams{
		UserID: user.ID,
		Query:  q,
		Limit:  limit,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Search query failed")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"query": q,
		"count": len(posts),
		"posts": posts,
	})
}

func (s *state) handleAPIBookmarks(w http.ResponseWriter, r *http.Request, user database.User) {
	switch r.Method {
	case http.MethodGet:
		limit := int32(20)
		if l := r.URL.Query().Get("limit"); l != "" {
			if val, err := strconv.Atoi(l); err == nil && val > 0 {
				limit = int32(val)
			}
		}

		bms, err := s.db.GetBookmarksForUser(r.Context(), database.GetBookmarksForUserParams{
			UserID: user.ID,
			Limit:  limit,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to load bookmarks")
			return
		}

		respondWithJSON(w, http.StatusOK, bms)

	case http.MethodPost:
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body, 'url' is required")
			return
		}

		post, err := s.db.GetPostByURL(r.Context(), req.URL)
		if err != nil {
			respondWithError(w, http.StatusNotFound, "Post not found with that URL")
			return
		}

		now := time.Now().UTC()
		bm, err := s.db.CreateBookmark(r.Context(), database.CreateBookmarkParams{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
			UserID:    user.ID,
			PostID:    post.ID,
		})
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
				respondWithError(w, http.StatusConflict, "Post already bookmarked")
				return
			}
			respondWithError(w, http.StatusInternalServerError, "Failed to create bookmark")
			return
		}

		respondWithJSON(w, http.StatusCreated, bm)

	case http.MethodDelete:
		url := r.URL.Query().Get("url")
		if url == "" {
			var req struct {
				URL string `json:"url"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			url = req.URL
		}
		if url == "" {
			respondWithError(w, http.StatusBadRequest, "'url' parameter required")
			return
		}

		post, err := s.db.GetPostByURL(r.Context(), url)
		if err != nil {
			respondWithError(w, http.StatusNotFound, "Post not found")
			return
		}

		err = s.db.DeleteBookmark(r.Context(), database.DeleteBookmarkParams{
			UserID: user.ID,
			PostID: post.ID,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to delete bookmark")
			return
		}

		respondWithJSON(w, http.StatusOK, map[string]string{"message": "Bookmark removed"})

	default:
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *state) handleAPIFollowing(w http.ResponseWriter, r *http.Request, user database.User) {
	switch r.Method {
	case http.MethodGet:
		follows, err := s.db.GetFeedFollowsForUser(r.Context(), user.ID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve followed feeds")
			return
		}
		respondWithJSON(w, http.StatusOK, follows)

	case http.MethodPost:
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body, 'url' is required")
			return
		}

		feed, err := s.db.GetFeedByURL(r.Context(), req.URL)
		if err != nil {
			respondWithError(w, http.StatusNotFound, "Feed not found with that URL")
			return
		}

		now := time.Now().UTC()
		follow, err := s.db.CreateFeedFollow(r.Context(), database.CreateFeedFollowParams{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
			UserID:    user.ID,
			FeedID:    feed.ID,
		})
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
				respondWithError(w, http.StatusConflict, "Feed already followed")
				return
			}
			respondWithError(w, http.StatusInternalServerError, "Failed to follow feed")
			return
		}

		respondWithJSON(w, http.StatusCreated, follow)

	default:
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handlerServe(s *state, cmd command) error {
	port := ":8080"
	if len(cmd.args) > 0 {
		p := cmd.args[0]
		if !strings.HasPrefix(p, ":") {
			p = ":" + p
		}
		port = p
	}

	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/api/feeds", s.handleAPIFeeds)

	// Authenticated endpoints
	mux.HandleFunc("/api/posts", s.authMiddleware(s.handleAPIPosts))
	mux.HandleFunc("/api/search", s.authMiddleware(s.handleAPISearch))
	mux.HandleFunc("/api/bookmarks", s.authMiddleware(s.handleAPIBookmarks))
	mux.HandleFunc("/api/following", s.authMiddleware(s.handleAPIFollowing))

	server := &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Printf("Gator HTTP REST API listening on http://localhost%s\n", port)
	fmt.Printf("Endpoints:\n")
	fmt.Printf("  GET  /healthz              - Health check\n")
	fmt.Printf("  GET  /api/feeds            - List public feeds\n")
	fmt.Printf("  GET  /api/posts            - Browse user posts (Auth required)\n")
	fmt.Printf("  GET  /api/search?q=...     - Search user posts (Auth required)\n")
	fmt.Printf("  GET  /api/bookmarks        - List bookmarks (Auth required)\n")
	fmt.Printf("  POST /api/bookmarks        - Add bookmark (Auth required)\n")
	fmt.Printf("  GET  /api/following        - List followed feeds (Auth required)\n")
	fmt.Printf("  POST /api/following        - Follow feed (Auth required)\n")

	return server.ListenAndServe()
}
