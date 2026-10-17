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
Clone the repository and run:
```bash
git clone https://github.com/<your-username>/<your-repo-name>.git
cd <your-repo-name>
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
Run the Goose migrations located in `sql/schema` to initialize the database tables (`users`, `feeds`, `feed_follows`, and `posts`):

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
- **Reset database** (deletes all users and cascades to all feeds, follows, and posts):
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
  Runs a continuous background loop scraping feeds in order of least recently fetched. Takes a duration string (`time_between_reqs` such as `30s`, `1m`, `5m`):
  ```bash
  gator agg 1m
  ```
  *(Press `Ctrl+C` to stop the aggregator loop).*

- **Browse latest posts from feeds you follow**:
  View posts in the terminal. Accepts an optional limit (defaults to 2):
  ```bash
  gator browse
  gator browse 10
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
