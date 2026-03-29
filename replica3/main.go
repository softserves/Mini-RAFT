package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func appendEntryHandler(w http.ResponseWriter, r *http.Request) {
	var msg map[string]interface{}

	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	fmt.Println("Received from gateway:", msg)

	// Simulate commit response
	response := map[string]interface{}{
		"type": "stroke",
		"points": msg["points"],
        "color": msg["color"],
        "width": msg["width"],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/append-entry", appendEntryHandler)

	fmt.Println("Replica 3 running on :9003")
	log.Fatal(http.ListenAndServe(":9003", nil))
}
