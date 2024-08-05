CREATE TABLE IF NOT EXISTS rooms (
  "id" uuid PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
  "name" VARCHAR(255) NOT NULL,
  "created_at" DATE NOT NULL DEFAULT now(),
  "updated_at" DATE NOT NULL DEFAULT now()
);

---- create above / drop below ----
DROP TABLE IF EXISTS rooms;