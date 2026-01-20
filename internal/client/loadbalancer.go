package client

import (
	"math/rand"
	"sync"
	"time"
)

// LoadBalancingAlgorithm rappresenta l'algoritmo di load balancing
type LoadBalancingAlgorithm int

const (
	RoundRobin LoadBalancingAlgorithm = iota
	Random
)

type LoadBalancer struct {
	servers    []*ServerInfo
	Index      int
	algorithm  LoadBalancingAlgorithm
	mu         sync.Mutex
	rng        *rand.Rand
	quorumSize int // Per quorum-based replication
}

// Contiene le informazioni di un server
type ServerInfo struct {
	Address string
	Port    int32
	ID      string // Server ID aggiunto per quorum tracking
}

// Crea un nuovo load balancer con l'algoritmo specificato
func NewLoadBalancer(algorithm LoadBalancingAlgorithm) *LoadBalancer {
	return &LoadBalancer{
		servers:    make([]*ServerInfo, 0),
		Index:      0,
		algorithm:  algorithm,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		quorumSize: 0,
	}
}

// Imposta la lista di server
func (lb *LoadBalancer) SetServers(servers []*ServerInfo) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.servers = make([]*ServerInfo, len(servers))
	copy(lb.servers, servers)
	lb.Index = 0

	// Calcola la dimensione del quorum: ceil(n/2) + 1 per garantire maiioranza
	// Ma per semplicità, se vogliamo quorum write, usiamo n (tutti)
	lb.quorumSize = len(servers)
}

// Ritorna il prossimo server usando l'algoritmo configurato
func (lb *LoadBalancer) GetNextServer() *ServerInfo {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.servers) == 0 {
		return nil
	}

	var server *ServerInfo

	switch lb.algorithm {
	case RoundRobin:
		server = lb.servers[lb.Index]
		lb.Index = (lb.Index + 1) % len(lb.servers)

	case Random:
		index := lb.rng.Intn(len(lb.servers))
		server = lb.servers[index]
	}

	return server
}

// ritorna tutti i server disponibili (per quorum operations)
func (lb *LoadBalancer) GetAllServers() []*ServerInfo {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	servers := make([]*ServerInfo, len(lb.servers))
	copy(servers, lb.servers)
	return servers
}

// ritorna la dimensione del quorum
func (lb *LoadBalancer) GetQuorumSize() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	return lb.quorumSize
}

// ritorna il numero di server disponibili
func (lb *LoadBalancer) GetServersCount() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	return len(lb.servers)
}
