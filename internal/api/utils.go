func (h apiHandler) readRoom(w http.ResponseWriter, r *http.Request) (room pgstore.GetRoomRow, rawRoomID string, roomId uuid.UUID, ok bool) {
	rawRoomID = chi.URLParam(r, "room_id")
	roomId, err := uuid.Parse(rawRoomID)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	room, err = h.q.GetRoom(r.Context(), roomId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "room not found", http.StatusBadRequest)
			return pgstore.GetRoomRow{}, "", uuid.UUID{}, false
		}

		slog.Error("error getting room", "room", roomId, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return pgstore.GetRoomRow{}, "", uuid.UUID{}, false
	}

	return room, rawRoomID, roomId, true
}

func sendJSON(w http.ResponseWriter, rawData any) {
	data, _ := json.Marshal(rawData)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
