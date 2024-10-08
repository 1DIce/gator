-- +goose Up
CREATE TABLE posts (
  id UUID PRIMARY KEY,
  url  TEXT NOT NULL,
  UNIQUE(url),

  title TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  description TEXT,
  published_at TIMESTAMP,

  feed_id UUID NOT NULL,
  CONSTRAINT fk_feed_id
  FOREIGN KEY (feed_id)
  REFERENCES feeds(id)
  ON DELETE CASCADE
);

-- +goose Down
DROP TABLE posts;
