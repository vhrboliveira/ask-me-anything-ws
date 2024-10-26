CREATE TABLE IF NOT EXISTS rooms (
  "id" BIGSERIAL PRIMARY KEY NOT NULL,
  "name" VARCHAR(255) NOT NULL,
  "created_at" TIMESTAMP NOT NULL DEFAULT now(),
  "updated_at" TIMESTAMP NOT NULL DEFAULT now()
);

---- create above / drop below ----

DROP TABLE IF EXISTS rooms;