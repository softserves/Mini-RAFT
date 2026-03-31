package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

func cloneStroke(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return map[string]interface{}{}
	}
	copyMap := make(map[string]interface{}, len(input))
	for k, v := range input {
		copyMap[k] = v
	}
	return copyMap
}

func (n *Node) appendLocalEntry(stroke map[string]interface{}) LogEntry {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	entry := LogEntry{
		Term:   n.currentTerm,
		Index:  len(n.log) + 1,
		Stroke: cloneStroke(stroke),
	}
	n.log = append(n.log, entry)
	n.matchIndex[n.id] = entry.Index
	n.nextIndex[n.id] = entry.Index + 1
	n.logf("appended entry index=%d", entry.Index)
	return entry
}

func (n *Node) initLeaderReplicationState() {
	next := len(n.log) + 1
	n.nextIndex = make(map[string]int)
	n.matchIndex = make(map[string]int)
	n.nextIndex[n.id] = next
	n.matchIndex[n.id] = len(n.log)
	for _, peer := range n.peers {
		n.nextIndex[peer] = next
		n.matchIndex[peer] = 0
	}
}

func (n *Node) buildAppendEntries(peer string, heartbeatOnly bool) (AppendEntriesRequest, int, int) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	nextIdx := n.nextIndex[peer]
	if nextIdx <= 0 {
		nextIdx = len(n.log) + 1
		n.nextIndex[peer] = nextIdx
	}

	prevLogIndex := nextIdx - 1
	prevLogTerm := 0
	if prevLogIndex > 0 && prevLogIndex <= len(n.log) {
		prevLogTerm = n.log[prevLogIndex-1].Term
	}

	entries := make([]LogEntry, 0)
	if !heartbeatOnly && nextIdx-1 < len(n.log) {
		entries = append(entries, n.log[nextIdx-1:]...)
	}

	if heartbeatOnly && nextIdx-1 < len(n.log) {
		entries = append(entries, n.log[nextIdx-1:]...)
	}

	req := AppendEntriesRequest{
		Term:         n.currentTerm,
		LeaderID:     n.id,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: n.commitIndex,
	}

	lastReplicated := prevLogIndex + len(entries)
	return req, lastReplicated, n.currentTerm
}

func (n *Node) replicateToPeer(peer string, requiredIndex int, heartbeatOnly bool) bool {
	attempts := 0
	for attempts < 8 {
		attempts++

		req, lastReplicated, sentTerm := n.buildAppendEntries(peer, heartbeatOnly)

		body, err := json.Marshal(req)
		if err != nil {
			return false
		}

		client := &http.Client{Timeout: 250 * time.Millisecond}
		resp, err := client.Post(peer+"/append-entries", "application/json", bytes.NewBuffer(body))
		if err != nil {
			return false
		}

		var appendResp AppendEntriesResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&appendResp)
		resp.Body.Close()
		if decodeErr != nil {
			return false
		}

		if appendResp.Term > sentTerm {
			n.stepDown(appendResp.Term)
			return false
		}

		if appendResp.Success {
			n.mutex.Lock()
			if lastReplicated > n.matchIndex[peer] {
				n.matchIndex[peer] = lastReplicated
			}
			if n.nextIndex[peer] < lastReplicated+1 {
				n.nextIndex[peer] = lastReplicated + 1
			}
			n.mutex.Unlock()

			if requiredIndex > 0 && lastReplicated >= requiredIndex {
				n.logf("replicated entry to %s", peerID(peer))
				return true
			}
			if requiredIndex == 0 {
				return true
			}
			continue
		}

		n.mutex.Lock()
		if n.nextIndex[peer] > 1 {
			n.nextIndex[peer]--
		} else {
			n.mutex.Unlock()
			return false
		}
		n.mutex.Unlock()
	}

	return false
}

func (n *Node) replicateForCommit(entryIndex int) bool {
	n.mutex.Lock()
	if n.state != StateLeader {
		n.mutex.Unlock()
		return false
	}
	peers := append([]string(nil), n.peers...)
	n.mutex.Unlock()

	acks := 1
	results := make(chan bool, len(peers))

	for _, peer := range peers {
		go func(p string) {
			results <- n.replicateToPeer(p, entryIndex, false)
		}(peer)
	}

	deadline := time.After(1200 * time.Millisecond)
	processed := 0
	for processed < len(peers) {
		select {
		case ok := <-results:
			processed++
			if ok {
				acks++
				if acks >= n.majority() {
					n.mutex.Lock()
					if entryIndex > n.commitIndex {
						n.commitIndex = entryIndex
						if n.lastApplied < n.commitIndex {
							n.lastApplied = n.commitIndex
						}
						n.logf("committed index=%d", entryIndex)
					}
					n.mutex.Unlock()
					return true
				}
			}
		case <-deadline:
			return false
		}
	}

	if acks >= n.majority() {
		n.mutex.Lock()
		if entryIndex > n.commitIndex {
			n.commitIndex = entryIndex
			if n.lastApplied < n.commitIndex {
				n.lastApplied = n.commitIndex
			}
			n.logf("committed index=%d", entryIndex)
		}
		n.mutex.Unlock()
		return true
	}

	return false
}
