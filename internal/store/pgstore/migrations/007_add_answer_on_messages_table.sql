ALTER TABLE messages 
  ADD COLUMN answer TEXT NOT NULL default '';

---- create above / drop below ----

ALTER TABLE messages
  DROP COLUMN answer;