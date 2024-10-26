package types

const (
	MessageKindMessageCreated         = "message_created"
	MessageKindMessageReactionAdd     = "message_reaction_added"
	MessageKindMessageReactionRemoved = "message_reaction_removed"
	MessageKindMessageAnswered        = "message_answered"
	MessageKindRoomCreated            = "room_created"
)

type MessageCreated struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

type MessageReactionAdded struct {
	ID    string `json:"id"`
	Count int32  `json:"count"`
}

type MessageReactionRemoved struct {
	ID    string `json:"id"`
	Count int32  `json:"count"`
}

type MessageAnswered struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
}

type Message struct {
	Kind   string `json:"kind"`
	Value  any    `json:"value"`
	RoomID int64  `json:"-"`
}

type RoomCreated struct {
	ID          int64  `json:"id"`
	CreatedAt   string `json:"created_at"`
	Name        string `json:"name"`
	UserID      string `json:"user_id"`
	CreatorName string `json:"creator_name"`
	Description string `json:"description"`
}
