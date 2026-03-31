package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var currentLeader = 0

func replicaTargets() []string {
	urls := []string{
		os.Getenv("REPLICA1_URL"),
		os.Getenv("REPLICA2_URL"),
		os.Getenv("REPLICA3_URL"),
	}

	fallback := []string{
		"http://replica1:9000",
		"http://replica2:9000",
		"http://replica3:9000",
	}

	result := make([]string, 0, 3)
	for i, u := range urls {
		if strings.TrimSpace(u) == "" {
			u = fallback[i]
		}
		u = strings.TrimRight(u, "/") + "/append-entry"
		result = append(result, u)
	}

	return result
}

func leaderIndexByID(leaderID string, replicas []string) int {
	if strings.TrimSpace(leaderID) == "" {
		return -1
	}

	for i, url := range replicas {
		if strings.Contains(url, leaderID+":") || strings.Contains(url, "/"+leaderID) {
			return i
		}
	}
	return -1
}

func sendToLeader(msg map[string]interface{}) {
	fmt.Println("Trying replicas...")

	replicas := replicaTargets()
	startIndex := currentLeader

	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("Marshal error:", err)
		return
	}

	for i := 0; i < len(replicas); i++ {
		index := (startIndex + i) % len(replicas)
		url := replicas[index]

		fmt.Println("Trying:", url)

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Println("Failed:", url)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusConflict {
			var errResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
				if leaderID, ok := errResp["leaderId"].(string); ok {
					if idx := leaderIndexByID(leaderID, replicas); idx >= 0 {
						currentLeader = idx
					}
				}
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			fmt.Println("Decode error:", err)
			continue
		}

		fmt.Println("Success from:", url)

		// ✅ Update current leader
		currentLeader = index

		// ✅ Broadcast to all clients
		for client := range clients {
			err := client.WriteJSON(response)
			if err != nil {
				log.Println("Write error:", err)
				client.Close()
				delete(clients, client)
			}
		}

		return
	}

	fmt.Println("All replicas failed")
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()

	clients[ws] = true
	fmt.Println("New client connected")

	for {
		var msg map[string]interface{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			delete(clients, ws)
			break
		}

		fmt.Println("Received:", msg)

		// Check message type
		msgType, ok := msg["type"].(string)
		if !ok {
			fmt.Println("Invalid message format")
			continue
		}

		if msgType == "stroke" {
			fmt.Println("Stroke received → forwarding to leader")
			sendToLeader(msg)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleConnections)

	fmt.Println("Gateway running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
