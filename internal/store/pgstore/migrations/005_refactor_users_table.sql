ALTER TABLE users
ADD COLUMN "photo" TEXT NOT NULL DEFAULT '',
ADD COLUMN "enable_picture" BOOLEAN NOT NULL DEFAULT TRUE,
ADD COLUMN "provider" VARCHAR(255) NOT NULL DEFAULT '',
ADD COLUMN "provider_user_id" VARCHAR(255) NOT NULL DEFAULT '',
ADD COLUMN "new_user" BOOLEAN NOT NULL DEFAULT TRUE,
DROP COLUMN "password_hash",
DROP COLUMN "bio";

---- create above / drop below ----

ALTER TABLE users
DROP COLUMN "photo",
DROP COLUMN "enable_picture",
DROP COLUMN "provider",
DROP COLUMN "provider_user_id",
DROP COLUMN "new_user",
ADD COLUMN "password_hash" VARCHAR(255) NOT NULL,
ADD COLUMN "bio" VARCHAR(255) NOT NULL DEFAULT '';