package main

import (
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const (
	StateFollower  = "Follower"
	StateCandidate = "Candidate"
	StateLeader    = "Leader"
)

type Node struct {
	id            string
	state         string
	currentTerm   int
	votedFor      string
	leaderId      string
	lastHeartbeat time.Time
	peers         []string
	votes         int
	mutex         sync.Mutex
}

type VoteRequest struct {
	Term        int    `json:"term"`
	CandidateID string `json:"candidateId"`
}

type VoteResponse struct {
	Term        int  `json:"term"`
	VoteGranted bool `json:"voteGranted"`
}

type HeartbeatRequest struct {
	Term     int    `json:"term"`
	LeaderID string `json:"leaderId"`
}

type HeartbeatResponse struct {
	Term    int  `json:"term"`
	Success bool `json:"success"`
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewNode(id string, peers []string) *Node {
	return &Node{
		id:            id,
		state:         StateFollower,
		currentTerm:   0,
		votedFor:      "",
		leaderId:      "",
		lastHeartbeat: time.Now(),
		peers:         peers,
		votes:         0,
	}
}

func (n *Node) logf(format string, args ...interface{}) {
	log.Printf("["+n.id+"] "+format, args...)
}

func (n *Node) majority() int {
	total := len(n.peers) + 1
	return total/2 + 1
}

func randomElectionTimeout() time.Duration {
	ms := 500 + rand.Intn(301)
	return time.Duration(ms) * time.Millisecond
}

func parsePeers(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	peers := make([]string, 0, len(parts))
	for _, part := range parts {
		peer := strings.TrimSpace(part)
		if peer == "" {
			continue
		}
		if !strings.HasPrefix(peer, "http://") && !strings.HasPrefix(peer, "https://") {
			peer = "http://" + peer
		}
		peers = append(peers, strings.TrimRight(peer, "/"))
	}
	return peers
}

func peerID(peerURL string) string {
	withoutScheme := strings.TrimPrefix(strings.TrimPrefix(peerURL, "http://"), "https://")
	host := strings.Split(withoutScheme, "/")[0]
	parts := strings.Split(host, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return host
}
