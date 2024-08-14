func sendJSON(w http.ResponseWriter, rawData any) {
	data, _ := json.Marshal(rawData)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
