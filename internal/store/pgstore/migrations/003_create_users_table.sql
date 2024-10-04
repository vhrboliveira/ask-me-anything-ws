CREATE TABLE IF NOT EXISTS users (
  "id" uuid PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  "email" VARCHAR(255) NOT NULL UNIQUE,
  "password_hash" VARCHAR(255) NOT NULL,
  "name" VARCHAR(255) NOT NULL,
  "bio" VARCHAR(255) NOT NULL DEFAULT '',
  "created_at" TIMESTAMP NOT NULL DEFAULT now(),
  "updated_at" TIMESTAMP NOT NULL DEFAULT now()
);

---- create above / drop below ----

DROP TABLE IF EXISTS users;