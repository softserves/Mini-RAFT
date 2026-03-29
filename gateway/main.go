package main

import (
    "bytes"
    "encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var currentLeader = 0

func sendToLeader(msg map[string]interface{}) {
	fmt.Println("Trying replicas...")

	replicas := []string{
		"http://replica1:9001/append-entry",
		"http://replica2:9002/append-entry",
		"http://replica3:9003/append-entry",
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("Marshal error:", err)
		return
	}

	for i := 0; i < len(replicas); i++ {
		index := (currentLeader + i) % len(replicas)
		url := replicas[index]

		fmt.Println("Trying:", url)

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Println("Failed:", url)
			continue
		}

		defer resp.Body.Close()

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
