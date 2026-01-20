package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/Flaasks/esercizio-go/pkg/pb"
	"google.golang.org/grpc"
)

// Implementa il servizio Counter con stato e replicazione quorum-based
type CounterServer struct {
	pb.UnimplementedCounterServiceServer

	ID      string
	counter int32
	version int64 // Versione dello stato per quorum
	mu      sync.Mutex
	peers   map[string]string // ID -> indirizzo peer
	peersMu sync.RWMutex
}

// Crea un nuovo server counter
func NewCounterServer(id string) *CounterServer {
	return &CounterServer{
		ID:      id,
		counter: 0,
		version: 0,
		peers:   make(map[string]string),
	}
}

// Registra un peer server per la replicazione quorum-based
func (cs *CounterServer) RegisterPeer(peerID, peerAddr string) {
	cs.peersMu.Lock()
	defer cs.peersMu.Unlock()
	cs.peers[peerID] = peerAddr
	fmt.Printf("[%s] Peer registrato: %s@%s\n", cs.ID, peerID, peerAddr)
}

// Incrementa il contatore con replicazione quorum
func (cs *CounterServer) Increment(ctx context.Context, req *pb.IncrementRequest) (*pb.CounterResponse, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	amount := req.GetAmount()
	if amount <= 0 {
		amount = 1
	}

	// Incrementa localmente
	cs.counter += int32(amount)
	cs.version++
	currentVersion := cs.version

	fmt.Printf("[%s] Increment(%d) -> Valore: %d, Versione: %d\n",
		cs.ID, amount, cs.counter, currentVersion)

	// Replica su tutti i peer (quorum = tutti i server)
	go cs.replicateToAll(currentVersion, "increment", amount)

	return &pb.CounterResponse{
		Value:    cs.counter,
		Version:  currentVersion,
		ServerId: cs.ID,
	}, nil
}

// Decrementa il contatore con replicazione quorum
func (cs *CounterServer) Decrement(ctx context.Context, req *pb.DecrementRequest) (*pb.CounterResponse, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	amount := req.GetAmount()
	if amount <= 0 {
		amount = 1
	}

	// Decrementa localmente
	cs.counter -= int32(amount)
	cs.version++
	currentVersion := cs.version

	fmt.Printf("[%s] Decrement(%d) -> Valore: %d, Versione: %d\n",
		cs.ID, amount, cs.counter, currentVersion)

	// Replica su tutti i peer (quorum = tutti i server)
	go cs.replicateToAll(currentVersion, "decrement", amount)

	return &pb.CounterResponse{
		Value:    cs.counter,
		Version:  currentVersion,
		ServerId: cs.ID,
	}, nil
}

// Restituisce il valore attuale del contatore
func (cs *CounterServer) GetValue(ctx context.Context, req *pb.GetValueRequest) (*pb.CounterResponse, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	fmt.Printf("[%s] GetValue() -> Valore: %d, Versione: %d\n",
		cs.ID, cs.counter, cs.version)

	return &pb.CounterResponse{
		Value:    cs.counter,
		Version:  cs.version,
		ServerId: cs.ID,
	}, nil
}

// implementa la replicazione increment da peer
func (cs *CounterServer) ReplicateIncrement(ctx context.Context, req *pb.ReplicationRequest) (*pb.ReplicationResponse, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Applica la modifica SOLO se la versione è più recente
	if req.GetVersion() > cs.version {
		cs.counter += int32(req.GetAmount())
		cs.version = req.GetVersion()
		fmt.Printf("[%s] ReplicaIncrement (remote) +%d -> Valore: %d, Versione: %d\n",
			cs.ID, req.GetAmount(), cs.counter, cs.version)
	} else {
		fmt.Printf("[%s] ReplicaIncrement IGNORATA (versione locale %d >= remota %d)\n",
			cs.ID, cs.version, req.GetVersion())
	}

	return &pb.ReplicationResponse{
		Success: true,
		Value:   cs.counter,
		Version: cs.version,
	}, nil
}

// implementa la replicazione decrement da peer
func (cs *CounterServer) ReplicateDecrement(ctx context.Context, req *pb.ReplicationRequest) (*pb.ReplicationResponse, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Applica la modifica solo se la versione è più recente
	if req.GetVersion() > cs.version {
		cs.counter -= int32(req.GetAmount())
		cs.version = req.GetVersion()
		fmt.Printf("[%s] ReplicaDecrement (remote) -%d -> Valore: %d, Versione: %d\n",
			cs.ID, req.GetAmount(), cs.counter, cs.version)
	} else {
		fmt.Printf("[%s] ReplicaDecrement IGNORATA (versione locale %d >= remota %d)\n",
			cs.ID, cs.version, req.GetVersion())
	}

	return &pb.ReplicationResponse{
		Success: true,
		Value:   cs.counter,
		Version: cs.version,
	}, nil
}

// invia la modifica a tutti i peer (quorum = n)
func (cs *CounterServer) replicateToAll(version int64, operation string, amount int32) {
	cs.peersMu.RLock()
	peers := make(map[string]string)
	for id, addr := range cs.peers {
		peers[id] = addr
	}
	cs.peersMu.RUnlock()

	if len(peers) == 0 {
		// Nessun peer, replicazione completata (solo questo server)
		return
	}

	// Invia a tutti i peer in parallelo
	var wg sync.WaitGroup
	var successCount int32

	for peerID, peerAddr := range peers {
		wg.Add(1)
		go func(id, addr string) {
			defer wg.Done()

			// Connessione con timeout
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(ctx, addr, grpc.WithInsecure())
			if err != nil {
				fmt.Printf("[%s] Errore connessione a %s: %v\n", cs.ID, id, err)
				return
			}
			defer conn.Close()

			client := pb.NewCounterServiceClient(conn)

			req := &pb.ReplicationRequest{
				Amount:    amount,
				Version:   version,
				Operation: operation,
			}

			var resp *pb.ReplicationResponse
			var err2 error

			if operation == "increment" {
				resp, err2 = client.ReplicateIncrement(ctx, req)
			} else {
				resp, err2 = client.ReplicateDecrement(ctx, req)
			}

			if err2 != nil {
				fmt.Printf("[%s] Errore replicazione a %s: %v\n", cs.ID, id, err2)
				return
			}

			if resp.GetSuccess() {
				atomic.AddInt32(&successCount, 1)
				fmt.Printf("[%s] Replica OK su %s (versione %d)\n", cs.ID, id, resp.GetVersion())
			}
		}(peerID, peerAddr)
	}

	wg.Wait()
	success := atomic.LoadInt32(&successCount)
	fmt.Printf("[%s] Replicazione completata: %d/%d peer aggiornati\n",
		cs.ID, success, len(peers))
}
