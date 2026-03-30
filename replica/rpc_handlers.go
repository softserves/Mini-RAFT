package main

import (
	"encoding/json"
	"net/http"
	"time"
)

func (n *Node) handleRequestVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if req.Term < n.currentTerm {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VoteResponse{Term: n.currentTerm, VoteGranted: false})
		return
	}

	if req.Term > n.currentTerm {
		n.currentTerm = req.Term
		n.state = StateFollower
		n.votedFor = ""
		n.leaderId = ""
		n.votes = 0
	}

	voteGranted := false
	if n.votedFor == "" || n.votedFor == req.CandidateID {
		n.votedFor = req.CandidateID
		n.lastHeartbeat = time.Now()
		voteGranted = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VoteResponse{Term: n.currentTerm, VoteGranted: voteGranted})
}

func (n *Node) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if req.Term < n.currentTerm {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HeartbeatResponse{Term: n.currentTerm, Success: false})
		return
	}

	if req.Term > n.currentTerm {
		n.currentTerm = req.Term
		n.votedFor = ""
	}

	n.state = StateFollower
	n.leaderId = req.LeaderID
	n.lastHeartbeat = time.Now()
	n.votes = 0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HeartbeatResponse{Term: n.currentTerm, Success: true})
}

func (n *Node) handleAppendEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	n.mutex.Lock()
	state := n.state
	leaderID := n.leaderId
	n.mutex.Unlock()

	if state != StateLeader {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":    "not_leader",
			"leaderId": leaderID,
		})
		return
	}

	response := map[string]interface{}{
		"type":   "stroke",
		"points": msg["points"],
		"color":  msg["color"],
		"width":  msg["width"],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *Node) handleAppendEntriesStub(w http.ResponseWriter, r *http.Request) {
	// TODO: Person 4 implementation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "not_implemented",
	})
}

func (n *Node) handleSyncLogStub(w http.ResponseWriter, r *http.Request) {
	// TODO: Person 4 implementation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "not_implemented",
	})
}
