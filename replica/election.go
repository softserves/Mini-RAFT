package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type voteResult struct {
	peer        string
	term        int
	voteGranted bool
}

func (n *Node) electionLoop() {
	lastObservedHeartbeat := time.Now()
	timeout := randomElectionTimeout()

	for {
		time.Sleep(25 * time.Millisecond)

		n.mutex.Lock()
		state := n.state
		lastHeartbeat := n.lastHeartbeat
		n.mutex.Unlock()

		if state == StateLeader {
			lastObservedHeartbeat = lastHeartbeat
			timeout = randomElectionTimeout()
			continue
		}

		if lastHeartbeat.After(lastObservedHeartbeat) {
			lastObservedHeartbeat = lastHeartbeat
			timeout = randomElectionTimeout()
		}

		if time.Since(lastObservedHeartbeat) >= timeout {
			n.startElection()
			n.mutex.Lock()
			lastObservedHeartbeat = n.lastHeartbeat
			n.mutex.Unlock()
			timeout = randomElectionTimeout()
		}
	}
}

func (n *Node) startElection() {
	n.mutex.Lock()
	if n.state == StateLeader {
		n.mutex.Unlock()
		return
	}

	n.state = StateCandidate
	n.currentTerm++
	term := n.currentTerm
	n.votedFor = n.id
	n.votes = 1
	n.leaderId = ""
	n.lastHeartbeat = time.Now()
	peers := append([]string(nil), n.peers...)
	n.mutex.Unlock()

	n.logf("starting election term=%d", term)

	voteCh := make(chan voteResult, len(peers))
	for _, peer := range peers {
		go n.sendRequestVote(peer, term, voteCh)
	}

	votes := 1
	responses := 0
	majority := n.majority()
	deadline := time.After(350 * time.Millisecond)

	for responses < len(peers) {
		select {
		case res := <-voteCh:
			responses++

			if res.term > term {
				n.stepDown(res.term)
				return
			}

			if res.voteGranted {
				votes++
				n.logf("vote granted by %s", res.peer)
				if votes >= majority {
					n.becomeLeader(term)
					return
				}
			}
		case <-deadline:
			return
		}
	}
}

func (n *Node) becomeLeader(term int) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if n.currentTerm != term || n.state != StateCandidate {
		return
	}

	n.state = StateLeader
	n.leaderId = n.id
	n.votes = 0
	n.lastHeartbeat = time.Now()
	n.initLeaderReplicationState()
	n.logf("became leader term=%d", term)
}

func (n *Node) stepDown(higherTerm int) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if higherTerm > n.currentTerm {
		n.currentTerm = higherTerm
	}
	n.state = StateFollower
	n.votedFor = ""
	n.leaderId = ""
	n.votes = 0
	n.lastHeartbeat = time.Now()
}

func (n *Node) heartbeatLoop() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		n.mutex.Lock()
		if n.state != StateLeader {
			n.mutex.Unlock()
			continue
		}
		peers := append([]string(nil), n.peers...)
		n.mutex.Unlock()

		for _, peer := range peers {
			go n.replicateToPeer(peer, 0, true)
		}
	}
}

func (n *Node) sendRequestVote(peer string, term int, voteCh chan<- voteResult) {
	payload := VoteRequest{Term: term, CandidateID: n.id}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Post(peer+"/request-vote", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var voteResp VoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&voteResp); err != nil {
		return
	}

	voteCh <- voteResult{
		peer:        peerID(peer),
		term:        voteResp.Term,
		voteGranted: voteResp.VoteGranted,
	}
}

func (n *Node) sendHeartbeat(peer string, term int) {
	_ = term
	n.replicateToPeer(peer, 0, true)
}
