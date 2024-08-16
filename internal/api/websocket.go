package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

func (h apiHandler) subscribeToRoom(w http.ResponseWriter, r *http.Request) {
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
	if _, ok := h.roomSubscribers[rawRoomID]; !ok {
		h.roomSubscribers[rawRoomID] = make(map[*websocket.Conn]context.CancelFunc)
	}
	slog.Info("new client connected", "room_id", rawRoomID, "client_IP", r.RemoteAddr)
	h.roomSubscribers[rawRoomID][c] = cancel
	h.mutex.Unlock()

	<-ctx.Done()

	h.mutex.Lock()
	delete(h.roomSubscribers[rawRoomID], c)
	h.mutex.Unlock()
}

func (h apiHandler) subscribeToRoomsList(w http.ResponseWriter, r *http.Request) {
	c, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("failed to upgrade connection", "error", err)
		http.Error(w, "failed to connect to ws connection", http.StatusBadRequest)
		return
	}

	defer c.Close()

	ctx, cancel := context.WithCancel(r.Context())

	h.mutex.Lock()
	slog.Info("new client connected to rooms list", "client_IP", r.RemoteAddr)
	h.roomsListSubscribers[c] = cancel
	h.mutex.Unlock()

	<-ctx.Done()

	h.mutex.Lock()
	delete(h.roomsListSubscribers, c)
	h.mutex.Unlock()
}

func (h apiHandler) notifyRoomClient(msg Message) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	subscribers, ok := h.roomSubscribers[msg.RoomID]
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

func (h apiHandler) notifyRoomsListClients(msg Message) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for conn, cancel := range h.roomsListSubscribers {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Error("failed to write room list message to client", "error", err)
			cancel()
		}
	}
}
