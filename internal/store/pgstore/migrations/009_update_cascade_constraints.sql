ALTER TABLE rooms
DROP CONSTRAINT rooms_user_id_fkey;

ALTER TABLE rooms
ADD CONSTRAINT fk_room_user_id
FOREIGN KEY (user_id) REFERENCES users(id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE messages
DROP CONSTRAINT messages_room_id_fkey;

ALTER TABLE messages
ADD CONSTRAINT fk_messages
FOREIGN KEY (room_id) REFERENCES rooms(id)
ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE messages_reactions 
DROP CONSTRAINT messages_reactions_message_id_fkey;

ALTER TABLE messages_reactions
ADD CONSTRAINT fk_messages_reactions_message_id
FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE ON UPDATE cascade;

ALTER TABLE messages_reactions 
DROP CONSTRAINT messages_reactions_user_id_fkey;

ALTER TABLE messages_reactions
ADD CONSTRAINT fk_messages_reactions_user_id
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE;

---- create above / drop below ----

ALTER TABLE rooms
ADD CONSTRAINT rooms_user_id_fkey
FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE rooms
DROP CONSTRAINT fk_rooms_user_id;

ALTER TABLE messages
ADD CONSTRAINT messages_room_id_fkey
FOREIGN KEY (room_id) REFERENCES rooms(id);

ALTER TABLE messages
DROP CONSTRAINT fk_messages;

ALTER TABLE messages_reactions 
ADD CONSTRAINT messages_reactions_message_id_fkey
FOREIGN KEY (message_id) REFERENCES messages(id);

ALTER TABLE messages_reactions
DROP CONSTRAINT fk_messages_reactions_message_id;

ALTER TABLE messages_reactions 
ADD CONSTRAINT messages_reactions_user_id_fkey
FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE messages_reactions
DROP CONSTRAINT fk_messages_reactions_user_id;