-- name: CreateBookmark :one
WITH inserted AS (
    INSERT INTO bookmarks (id, created_at, updated_at, user_id, post_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING *
)
SELECT inserted.*, posts.title AS post_title, posts.url AS post_url
FROM inserted
JOIN posts ON inserted.post_id = posts.id;

-- name: DeleteBookmark :exec
DELETE FROM bookmarks
WHERE user_id = $1 AND post_id = $2;

-- name: GetBookmarksForUser :many
SELECT bookmarks.*, posts.title AS post_title, posts.url AS post_url, posts.published_at, posts.description
FROM bookmarks
JOIN posts ON bookmarks.post_id = posts.id
WHERE bookmarks.user_id = $1
ORDER BY bookmarks.created_at DESC
LIMIT $2;

-- name: GetBookmarkByUserAndPost :one
SELECT * FROM bookmarks
WHERE user_id = $1 AND post_id = $2;
