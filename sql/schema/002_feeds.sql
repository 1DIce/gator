-- +goose Up
CREATE TABLE feeds (
  id UUID PRIMARY KEY,
  url TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  user_id UUID NOT NULL,
  CONSTRAINT fk_user_id
  FOREIGN KEY(user_id)
  REFERENCES users(id)
  ON DELETE CASCADE
);

-- +goose Down
DROP TABLE feeds;
