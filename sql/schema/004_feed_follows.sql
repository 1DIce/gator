-- +goose Up
CREATE TABLE feed_follows  (
  id UUID PRIMARY KEY,

  feed_id UUID NOT NULL,
  CONSTRAINT fk_feed_id
  FOREIGN KEY(feed_id)
  REFERENCES feeds(id)
  ON DELETE CASCADE,

  user_id UUID NOT NULL,
  CONSTRAINT fk_user_id
  FOREIGN KEY(user_id)
  REFERENCES users(id)
  ON DELETE CASCADE,

  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,

  UNIQUE(feed_id, user_id)
);

-- +goose Down
DROP TABLE feed_follows ;
