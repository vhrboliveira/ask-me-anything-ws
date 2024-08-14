package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

func (h apiHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	c, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("failed to upgrade connection", "error", err)
		http.Error(w, "failed to connect to ws connection", http.StatusBadRequest)
		return
	}

	defer c.Close()

	ctx, cancel := context.WithCancel(r.Context())

	h.mutex.Lock()
	if _, ok := h.subscribers[rawRoomID]; !ok {
		h.subscribers[rawRoomID] = make(map[*websocket.Conn]context.CancelFunc)
	}
	slog.Info("new client connected", "room_id", rawRoomID, "client_IP", r.RemoteAddr)
	h.subscribers[rawRoomID][c] = cancel
	h.mutex.Unlock()

	<-ctx.Done()

	h.mutex.Lock()
	delete(h.subscribers[rawRoomID], c)
	h.mutex.Unlock()
}

func (h apiHandler) notifyClient(msg Message) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	subscribers, ok := h.subscribers[msg.RoomID]
	if !ok || len(subscribers) == 0 {
		return
	}

	for conn, cancel := range subscribers {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Error("failed to write message to client", "error", err)
			cancel()
		}
	}
}
