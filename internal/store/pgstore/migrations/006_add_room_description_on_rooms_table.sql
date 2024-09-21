ALTER TABLE rooms
ADD COLUMN description VARCHAR(255) NOT NULL DEFAULT '';

---- create above / drop below ----
ALTER TABLE rooms
DROP COLUMN description;