ALTER TABLE rooms
ADD COLUMN user_id uuid NOT NULL,
ADD FOREIGN KEY (user_id) REFERENCES users(id);

---- create above / drop below ----

ALTER TABLE rooms
DROP COLUMN user_id;