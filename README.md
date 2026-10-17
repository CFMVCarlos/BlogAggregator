# gator

`gator` is a multi-user command-line RSS feed aggregator written in Go and backed by PostgreSQL. It allows multiple users to register, subscribe to RSS feeds, automatically collect and store posts in the background, and browse recent articles directly from the terminal.

---

## Prerequisites

Before running or installing `gator`, ensure the following dependencies are installed and running on your system:

- **Go**: Version 1.23 or newer ([Download Go](https://go.dev/dl/))
- **PostgreSQL**: Version 14 or newer ([Download PostgreSQL](https://www.postgresql.org/download/))
- **Goose** (optional, for applying database migrations):
  ```bash
  go install github.com/pressly/goose/v3/cmd/goose@latest
  ```

Ensure your PostgreSQL service is started:
```bash
sudo service postgresql start
# or on systemd-enabled systems:
sudo systemctl start postgresql
```

Create the `gator` database:
```bash
createdb gator
```

---

## Installation

You can install the compiled `gator` CLI directly into your `$GOPATH/bin` using Go:

### From Source
Clone the repository and install the binary:
```bash
git clone https://github.com/CFMVCarlos/BlogAggregator.git
cd BlogAggregator
go install .
```

Ensure `$HOME/go/bin` (or `$GOPATH/bin`) is in your system's `PATH`:
```bash
export PATH="$HOME/go/bin:$PATH"
```

Verify the installation:
```bash
gator
```

---

## Configuration & Database Setup

### 1. Configuration File
`gator` expects a configuration file located at `~/.gatorconfig.json` in your home directory. Create or edit this file with your PostgreSQL connection URL:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable",
  "current_user_name": ""
}
```

Adjust the username, password, host, and port as needed for your local PostgreSQL setup.

### 2. Database Migrations
Run the Goose migrations located in `sql/schema` to initialize the database tables (`users`, `feeds`, `feed_follows`, `posts`, and `bookmarks`):

```bash
goose -dir sql/schema postgres "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable" up
```

---

## Command Reference

### User Management
- **Register a new user** (automatically logs you in):
  ```bash
  gator register <name>
  ```
- **Login as an existing user**:
  ```bash
  gator login <name>
  ```
- **List all registered users**:
  ```bash
  gator users
  ```
- **Reset database** (deletes all users and cascades to all feeds, follows, posts, and bookmarks):
  ```bash
  gator reset
  ```

### Feed Management
- **Add a new RSS feed** (creates the feed and automatically follows it for the current user):
  ```bash
  gator addfeed "Hacker News RSS" "https://hnrss.org/newest"
  ```
- **List all feeds in the system**:
  ```bash
  gator feeds
  ```
- **Follow an existing feed by URL**:
  ```bash
  gator follow "https://hnrss.org/newest"
  ```
- **List feeds followed by current user**:
  ```bash
  gator following
  ```
- **Unfollow a feed by URL**:
  ```bash
  gator unfollow "https://hnrss.org/newest"
  ```

### Aggregator & Browsing Posts
- **Run the feed aggregator**:
  Runs a continuous background loop scraping feeds concurrently in order of least recently fetched. Accepts a duration string and an optional worker count (defaults to 3):
  ```bash
  gator agg 1m
  gator agg 30s 5
  ```
  *(Press `Ctrl+C` to stop the aggregator loop).*

- **Browse latest posts with pagination & filtering**:
  View posts in the terminal. Accepts an optional limit (default 2), page offset (default 1), and optional feed URL filter:
  ```bash
  gator browse
  gator browse 5 1
  gator browse 5 2 "https://hnrss.org/newest"
  ```

- **Search posts by keyword**:
  Search through followed feed articles by title or description:
  ```bash
  gator search "Go" 10
  ```

### Bookmarks & Favorites
- **Bookmark a post by URL**:
  ```bash
  gator bookmark "https://example.com/post-url"
  ```
- **Unbookmark a post by URL**:
  ```bash
  gator unbookmark "https://example.com/post-url"
  ```
- **List bookmarked posts**:
  ```bash
  gator bookmarks
  gator bookmarks 20
  ```

### Interactive Terminal User Interface (TUI)
- **Launch interactive terminal reader**:
  Interactive TUI with scrollable posts, live preview pane, and keybindings:
  ```bash
  gator tui
  gator tui 100
  ```
  **Keybindings**:
  - `↑` / `k`: Move up
  - `↓` / `j`: Move down
  - `Enter` / `o`: Open selected article in default web browser
  - `b`: Toggle bookmark on selected article
  - `q` / `Esc`: Quit reader

### HTTP REST API Server
- **Run the API server**:
  Starts an embedded JSON HTTP REST API server with authentication:
  ```bash
  gator serve
  gator serve :8080
  ```
  **Endpoints**:
  - `GET  /healthz`: Health check endpoint.
  - `GET  /api/feeds`: List all feeds.
  - `GET  /api/posts`: Browse followed feed posts (`Authorization: Bearer <username>`).
  - `GET  /api/search?q=...`: Search followed feed posts (`Authorization: Bearer <username>`).
  - `GET  /api/bookmarks`: List bookmarks (`Authorization: Bearer <username>`).
  - `POST /api/bookmarks`: Bookmark a post by URL (`{"url": "..."}`).
  - `GET  /api/following`: List followed feeds (`Authorization: Bearer <username>`).
  - `POST /api/following`: Follow a feed by URL (`{"url": "..."}`).

### Background Service Manager (Daemon & Auto-Restart)
- **Manage the background aggregator service**:
  Keeps `gator agg` running in the background and automatically restarts it with exponential backoff if it crashes:
  ```bash
  gator service start 1m 3    # Start background aggregator (duration & workers)
  gator service status        # Check if running and inspect recent logs
  gator service logs -f       # Stream/follow service logs
  gator service stop          # Gracefully terminate the background daemon
  ```

---

## Development

To run the project directly during development without compiling:
```bash
go run . <command> [args...]
```

To regenerate SQL queries via [sqlc](https://sqlc.dev/):
```bash
sqlc generate
```
