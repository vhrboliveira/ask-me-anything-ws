package web

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

func (h Handlers) SubscribeToRoom(w http.ResponseWriter, r *http.Request) {
	_, rawRoomID, _, ok := h.readRoom(w, r)
	if !ok {
		return
	}

	c, err := h.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("failed to upgrade connection", "error", err)
		http.Error(w, "failed to connect to ws connection", http.StatusBadRequest)
		return
	}

	defer c.Close()

	ctx, cancel := context.WithCancel(r.Context())

	h.Mutex.Lock()
	if _, ok := h.RoomSubscribers[rawRoomID]; !ok {
		h.RoomSubscribers[rawRoomID] = make(map[*websocket.Conn]context.CancelFunc)
	}
	slog.Info("new client connected", "room_id", rawRoomID, "client_IP", r.RemoteAddr)
	h.RoomSubscribers[rawRoomID][c] = cancel
	h.Mutex.Unlock()

	<-ctx.Done()

	h.Mutex.Lock()
	delete(h.RoomSubscribers[rawRoomID], c)
	h.Mutex.Unlock()
}

func (h Handlers) SubscribeToRoomsList(w http.ResponseWriter, r *http.Request) {
	c, err := h.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("failed to upgrade connection", "error", err)
		http.Error(w, "failed to connect to ws connection", http.StatusBadRequest)
		return
	}

	defer c.Close()

	ctx, cancel := context.WithCancel(r.Context())

	h.Mutex.Lock()
	slog.Info("new client connected to rooms list", "client_IP", r.RemoteAddr)
	h.RoomsListSubscribers[c] = cancel
	h.Mutex.Unlock()

	<-ctx.Done()

	h.Mutex.Lock()
	delete(h.RoomsListSubscribers, c)
	h.Mutex.Unlock()
}

func (h Handlers) NotifyRoomClient(msg Message) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	subscribers, ok := h.RoomSubscribers[msg.RoomID]
	if !ok || len(subscribers) == 0 {
		return
	}

	for conn, cancel := range subscribers {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Error("failed to write room message to client", "error", err)
			cancel()
		}
	}
}

func (h Handlers) NotifyRoomsListClients(msg Message) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	for conn, cancel := range h.RoomsListSubscribers {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Error("failed to write room list message to client", "error", err)
			cancel()
		}
	}
}
