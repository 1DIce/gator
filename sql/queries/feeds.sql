-- name: CreateFeed :one
INSERT INTO feeds (id, url,name,created_at, updated_at, user_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds
WHERE url = $1 LIMIT 1;

-- name: ListFeeds :many
SELECT feeds.url, feeds.name, users.name as user_name from feeds
INNER JOIN users ON feeds.user_id = users.id;

-- name: MarkFeedFetched :one
UPDATE feeds
SET last_fetched_at = $2, updated_at = $2
WHERE id = $1
RETURNING *;
