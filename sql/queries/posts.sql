-- name: CreatePost :one
INSERT INTO posts (id, url, title, created_at, updated_at, description, published_at, feed_id)
VALUES(
  $1,
  $2,
  $3,
  $4,
  $4,
  $5,
  $6,
  $7
)
RETURNING *;

-- name: GetPostsForUser :many
SELECT posts.* FROM posts
INNER JOIN feed_follows
ON posts.feed_id = feed_follows.feed_id
WHERE feed_follows.user_id = $1
ORDER BY published_at DESC
LIMIT $2;

