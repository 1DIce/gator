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

-- name: ListFeeds :many
SELECT feeds.url, feeds.name, users.name as user_name from feeds
INNER JOIN users ON feeds.user_id = users.id;
