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

	entry := n.appendLocalEntry(msg)
	if !n.replicateForCommit(entry.Index) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "replication_failed",
		})
		return
	}

	response := map[string]interface{}{
		"type":   "stroke",
		"points": entry.Stroke["points"],
		"color":  entry.Stroke["color"],
		"width":  entry.Stroke["width"],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *Node) handleAppendEntriesStub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AppendEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if req.Term < n.currentTerm {
		json.NewEncoder(w).Encode(AppendEntriesResponse{Term: n.currentTerm, Success: false})
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

	if req.PrevLogIndex > len(n.log) {
		json.NewEncoder(w).Encode(AppendEntriesResponse{Term: n.currentTerm, Success: false})
		return
	}

	if req.PrevLogIndex > 0 {
		if n.log[req.PrevLogIndex-1].Term != req.PrevLogTerm {
			json.NewEncoder(w).Encode(AppendEntriesResponse{Term: n.currentTerm, Success: false})
			return
		}
	}

	base := req.PrevLogIndex
	for i, incoming := range req.Entries {
		target := base + i + 1
		incoming.Index = target

		if target <= len(n.log) {
			if n.log[target-1].Term != incoming.Term {
				n.log = n.log[:target-1]
				n.log = append(n.log, incoming)
			} else {
				n.log[target-1].Stroke = cloneStroke(incoming.Stroke)
			}
		} else {
			n.log = append(n.log, LogEntry{
				Term:   incoming.Term,
				Index:  target,
				Stroke: cloneStroke(incoming.Stroke),
			})
		}
	}

	if req.LeaderCommit > n.commitIndex {
		if req.LeaderCommit < len(n.log) {
			n.commitIndex = req.LeaderCommit
		} else {
			n.commitIndex = len(n.log)
		}
		if n.lastApplied < n.commitIndex {
			n.lastApplied = n.commitIndex
		}
	}

	json.NewEncoder(w).Encode(AppendEntriesResponse{Term: n.currentTerm, Success: true})
}

func (n *Node) handleSyncLogStub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SyncLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	n.mutex.Lock()
	state := n.state
	leaderID := n.leaderId
	entries := make([]LogEntry, 0)
	if req.FromIndex < len(n.log) {
		entries = append(entries, n.log[req.FromIndex:]...)
	}
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SyncLogResponse{
		Entries: entries,
	})
}
