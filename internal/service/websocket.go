package service

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	types "github.com/vhrboliveira/ama-go/internal/utils"
)

type WebSocketService struct {
	Upgrader             websocket.Upgrader
	RoomSubscribers      map[string]map[*websocket.Conn]context.CancelFunc
	RoomsListSubscribers map[*websocket.Conn]context.CancelFunc
	Mutex                *sync.RWMutex
}

func NewWebSocketService() *WebSocketService {
	url := os.Getenv("SITE_URL")
	if url == "" {
		panic("SITE_URL is not set")
	}

	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "dev"
	}

	return &WebSocketService{
		Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
			if env != "production" {
				return true
			}
			return r.Header.Get("Origin") == url
		}},
		RoomSubscribers:      make(map[string]map[*websocket.Conn]context.CancelFunc),
		RoomsListSubscribers: make(map[*websocket.Conn]context.CancelFunc),
		Mutex:                &sync.RWMutex{},
	}
}

func (w *WebSocketService) SubscribeToRoom(c *websocket.Conn, ctx context.Context, cancel context.CancelFunc, roomID string, ip string) {
	w.Mutex.Lock()
	if _, ok := w.RoomSubscribers[roomID]; !ok {
		w.RoomSubscribers[roomID] = make(map[*websocket.Conn]context.CancelFunc)
	}
	slog.Info("new client connected", "room_id", roomID, "client_IP", ip)
	w.RoomSubscribers[roomID][c] = cancel
	w.Mutex.Unlock()

	<-ctx.Done()

	w.Mutex.Lock()
	delete(w.RoomSubscribers[roomID], c)
	slog.Info("client disconnected", "room_id", roomID, "client_IP", ip)
	w.Mutex.Unlock()
}

func (w *WebSocketService) SubscribeToRoomsList(c *websocket.Conn, ctx context.Context, cancel context.CancelFunc, ip string) {
	w.Mutex.Lock()
	slog.Info("new client connected to rooms list", "client_IP", ip)
	w.RoomsListSubscribers[c] = cancel
	w.Mutex.Unlock()

	<-ctx.Done()

	w.Mutex.Lock()
	delete(w.RoomsListSubscribers, c)
	slog.Info("client disconnected to rooms list", "client_IP", ip)
	w.Mutex.Unlock()
}

func (w *WebSocketService) NotifyRoomClient(msg types.Message) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	subscribers, ok := w.RoomSubscribers[msg.RoomID]
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

func (w *WebSocketService) NotifyRoomsListClients(msg types.Message) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	for conn, cancel := range w.RoomsListSubscribers {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Error("failed to write room list message to client", "error", err)
			cancel()
		}
	}
}
