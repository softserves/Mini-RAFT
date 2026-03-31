package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	id := os.Getenv("REPLICA_ID")
	if id == "" {
		id = "replica"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	node := NewNode(id, parsePeers(os.Getenv("PEERS")))

	go node.electionLoop()
	go node.heartbeatLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/append-entry", node.handleAppendEntry)
	mux.HandleFunc("/request-vote", node.handleRequestVote)
	mux.HandleFunc("/heartbeat", node.handleHeartbeat)
	mux.HandleFunc("/append-entries", node.handleAppendEntriesStub)
	mux.HandleFunc("/sync-log", node.handleSyncLogStub)

	addr := ":" + port
	log.Printf("[%s] running on %s", id, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
