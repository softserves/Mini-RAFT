package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)
var clientsMutex sync.RWMutex

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var currentLeader = 0
var leaderMutex sync.RWMutex

func getCurrentLeader() int {
	leaderMutex.RLock()
	defer leaderMutex.RUnlock()
	return currentLeader
}

func setCurrentLeader(index int) {
	leaderMutex.Lock()
	currentLeader = index
	leaderMutex.Unlock()
}

func clientCount() int {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()
	return len(clients)
}

func connectedClients() []*websocket.Conn {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()

	list := make([]*websocket.Conn, 0, len(clients))
	for client := range clients {
		list = append(list, client)
	}
	return list
}

func isReplicaAlive(baseURL string) bool {
	client := &http.Client{Timeout: 250 * time.Millisecond}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/request-vote", nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}

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
	startIndex := getCurrentLeader()

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
						setCurrentLeader(idx)
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
		setCurrentLeader(index)

		// ✅ Broadcast to all clients
		for _, client := range connectedClients() {
			err := client.WriteJSON(response)
			if err != nil {
				log.Println("Write error:", err)
				client.Close()
				clientsMutex.Lock()
				delete(clients, client)
				clientsMutex.Unlock()
			}
		}

		return
	}

	fmt.Println("All replicas failed")
}

func handleDebugStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	replicas := replicaTargets()
	type replicaStatus struct {
		URL   string `json:"url"`
		Alive bool   `json:"alive"`
	}

	statuses := make([]replicaStatus, 0, len(replicas))
	aliveCount := 0
	for _, appendURL := range replicas {
		baseURL := strings.TrimSuffix(appendURL, "/append-entry")
		alive := isReplicaAlive(baseURL)
		if alive {
			aliveCount++
		}
		statuses = append(statuses, replicaStatus{URL: baseURL, Alive: alive})
	}

	leaderIndex := getCurrentLeader()
	leaderURL := "unknown"
	if leaderIndex >= 0 && leaderIndex < len(replicas) {
		leaderURL = replicas[leaderIndex]
		if !isReplicaAlive(strings.TrimSuffix(leaderURL, "/append-entry")) {
			leaderURL = "unknown"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clientsConnected": clientCount(),
		"currentLeaderUrl": leaderURL,
		"replicas":         replicas,
		"replicaStatus":    statuses,
		"aliveReplicas":    aliveCount,
		"timestamp":        time.Now().Format(time.RFC3339),
	})
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer ws.Close()

	clientsMutex.Lock()
	clients[ws] = true
	clientsMutex.Unlock()
	fmt.Println("New client connected")

	for {
		var msg map[string]interface{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			clientsMutex.Lock()
			delete(clients, ws)
			clientsMutex.Unlock()
			break
		}

		fmt.Println("Received:", msg)

		// Check message type
		msgType, ok := msg["type"].(string)
		if !ok {
			fmt.Println("Invalid message format")
			continue
		}

		if msgType == "stroke" || msgType == "clear" {
			fmt.Printf("%s received → forwarding to leader\n", msgType)
			sendToLeader(msg)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/debug/stats", handleDebugStats)

	fmt.Println("Gateway running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
