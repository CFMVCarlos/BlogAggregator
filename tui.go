package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"BlogAggregator/internal/database"
)

func stripHTML(content string) string {
	var b strings.Builder
	inTag := false
	for _, r := range content {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	clean := strings.TrimSpace(b.String())
	clean = strings.ReplaceAll(clean, "\n\n", " ")
	clean = strings.ReplaceAll(clean, "\n", " ")
	return clean
}

func openURLInBrowser(targetURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", targetURL)
	default:
		if _, err := exec.LookPath("wslview"); err == nil {
			cmd = exec.Command("wslview", targetURL)
		} else if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", targetURL)
		} else {
			return
		}
	}
	_ = cmd.Start()
}

func handlerTUI(s *state, cmd command, user database.User) error {
	limit := 50
	if len(cmd.args) > 0 {
		l, err := strconv.Atoi(cmd.args[0])
		if err == nil && l > 0 {
			limit = l
		}
	}

	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	})
	if err != nil {
		return fmt.Errorf("could not load posts: %w", err)
	}

	if len(posts) == 0 {
		fmt.Println("No posts found. Follow some feeds and run 'agg' to populate posts.")
		return nil
	}

	// Put terminal in raw mode
	rawCmd := exec.Command("stty", "raw", "-echo")
	rawCmd.Stdin = os.Stdin
	_ = rawCmd.Run()

	defer func() {
		restoreCmd := exec.Command("stty", "-raw", "echo")
		restoreCmd.Stdin = os.Stdin
		_ = restoreCmd.Run()
		// Clear screen on exit
		fmt.Print("\033[H\033[2J")
	}()

	cursor := 0
	reader := bufio.NewReader(os.Stdin)
	statusMsg := "Ready"

	for {
		// Clear screen
		fmt.Print("\033[H\033[2J")

		// Header
		fmt.Printf("\r\033[1;36m=== GATOR TERMINAL READER ===\033[0m | User: \033[1;32m%s\033[0m | Total: %d posts\r\n", user.Name, len(posts))
		fmt.Println("\r--------------------------------------------------------------------------------")

		// View window of 10 items
		windowSize := 10
		start := cursor - windowSize/2
		if start < 0 {
			start = 0
		}
		end := start + windowSize
		if end > len(posts) {
			end = len(posts)
			start = end - windowSize
			if start < 0 {
				start = 0
			}
		}

		for i := start; i < end; i++ {
			post := posts[i]
			dateStr := "Recent    "
			if post.PublishedAt.Valid {
				dateStr = post.PublishedAt.Time.Format("Jan 02 15:04")
			}

			title := post.Title
			if len(title) > 60 {
				title = title[:57] + "..."
			}

			// Check if bookmarked
			bm, _ := s.db.GetBookmarkByUserAndPost(context.Background(), database.GetBookmarkByUserAndPostParams{
				UserID: user.ID,
				PostID: post.ID,
			})
			bmTag := " "
			if bm.ID != uuid.Nil {
				bmTag = "★"
			}

			if i == cursor {
				fmt.Printf("\r\033[1;7m > [%s] %s %-60s \033[0m\r\n", dateStr, bmTag, title)
			} else {
				fmt.Printf("\r   [%s] %s %-60s\r\n", dateStr, bmTag, title)
			}
		}

		// Preview pane
		selected := posts[cursor]
		fmt.Println("\r--------------------------------------------------------------------------------")
		fmt.Printf("\r\033[1;33mTitle:\033[0m       %s\r\n", selected.Title)
		fmt.Printf("\r\033[1;34mLink:\033[0m        %s\r\n", selected.Url)
		if selected.PublishedAt.Valid {
			fmt.Printf("\r\033[1;35mPublished:\033[0m   %s\r\n", selected.PublishedAt.Time.Format("2006-01-02 15:04:05 MST"))
		}
		if selected.Description.Valid && selected.Description.String != "" {
			desc := stripHTML(selected.Description.String)
			if len(desc) > 200 {
				desc = desc[:197] + "..."
			}
			fmt.Printf("\r\033[1mSummary:\033[0m     %s\r\n", desc)
		}

		// Footer & Keybindings
		fmt.Println("\r--------------------------------------------------------------------------------")
		fmt.Printf("\r\033[1;30;47m [↑/k] Up  [↓/j] Down  [Enter/o] Open in Browser  [b] Bookmark  [q] Quit \033[0m  %s\r\n", statusMsg)
		statusMsg = ""

		// Read key
		b, err := reader.ReadByte()
		if err != nil {
			break
		}

		if b == 'q' || b == 3 { // 'q' or Ctrl+C
			break
		} else if b == 'j' {
			if cursor < len(posts)-1 {
				cursor++
			}
		} else if b == 'k' {
			if cursor > 0 {
				cursor--
			}
		} else if b == '\r' || b == '\n' || b == 'o' {
			openURLInBrowser(selected.Url)
			statusMsg = fmt.Sprintf("Opened: %s", selected.Title)
		} else if b == 'b' {
			// Toggle bookmark
			existing, getErr := s.db.GetBookmarkByUserAndPost(context.Background(), database.GetBookmarkByUserAndPostParams{
				UserID: user.ID,
				PostID: selected.ID,
			})
			if getErr == nil && existing.ID != uuid.Nil {
				_ = s.db.DeleteBookmark(context.Background(), database.DeleteBookmarkParams{
					UserID: user.ID,
					PostID: selected.ID,
				})
				statusMsg = "Bookmark removed."
			} else {
				now := time.Now().UTC()
				_, _ = s.db.CreateBookmark(context.Background(), database.CreateBookmarkParams{
					ID:        uuid.New(),
					CreatedAt: now,
					UpdatedAt: now,
					UserID:    user.ID,
					PostID:    selected.ID,
				})
				statusMsg = "★ Bookmarked!"
			}
		} else if b == '\033' { // Escape sequence
			// Check next bytes
			b2, err := reader.ReadByte()
			if err == nil && b2 == '[' {
				b3, err := reader.ReadByte()
				if err == nil {
					switch b3 {
					case 'A': // Up arrow
						if cursor > 0 {
							cursor--
						}
					case 'B': // Down arrow
						if cursor < len(posts)-1 {
							cursor++
						}
					}
				}
			} else {
				// Standalone Esc quits
				break
			}
		}
	}

	return nil
}
