package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/Flaasks/esercizio-go/internal/server"
	pb "github.com/Flaasks/esercizio-go/pkg/pb"
)

const (
	basePort = 6000
)

var (
	serverID = flag.String("id", "server-1", "ID univoco del server")
)

func getRegistryAddress() string {
	if addr := os.Getenv("REGISTRY_ADDRESS"); addr != "" {
		return addr
	}
	return "localhost:5000"
}

func getServerHost() string {
	if host := os.Getenv("SERVER_HOST"); host != "" {
		return host
	}
	return "localhost"
}

func main() {
	flag.Parse()

	// Determina la porta in base all'ID
	var port int
	if *serverID == "server-1" {
		port = basePort
	} else if *serverID == "server-2" {
		port = basePort + 1
	} else {
		// Estrae il numero dall'ID
		var serverNum int
		fmt.Sscanf(*serverID, "server-%d", &serverNum)
		port = basePort + serverNum - 1
		if port < basePort {
			port = basePort + 1
		}
	}

	// Registra il server nel registry
	serverHost := getServerHost()
	if err := registerWithRegistry(*serverID, serverHost, int32(port)); err != nil {
		fmt.Printf("Errore nella registrazione: %v\n", err)
		return
	}

	// Crea il listener
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", serverHost, port))
	if err != nil {
		fmt.Printf("Errore nella creazione del listener: %v\n", err)
		deregisterWithRegistry(*serverID, serverHost, int32(port))
		return
	}
	defer listener.Close()

	// Crea il server gRPC
	grpcServer := grpc.NewServer()

	// Registra il servizio Counter
	counterServer := server.NewCounterServer(*serverID)
	pb.RegisterCounterServiceServer(grpcServer, counterServer)

	// Scopri altri server per la replicazione quorum-based
	time.Sleep(1 * time.Second) // Attende che il registry sia aggiornato
	if err := discoverAndRegisterPeers(*serverID, counterServer); err != nil {
		fmt.Printf("Avviso: errore nella scoperta dei peer: %v\n", err)
	}

	fmt.Printf("Algoritmo: Quorum-based Replication\n")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Printf("\n[%s] Shutdown in corso...\n", *serverID)
		grpcServer.GracefulStop()

		// Deregistra dal registry
		if err := deregisterWithRegistry(*serverID, serverHost, int32(port)); err != nil {
			fmt.Printf("[%s] Errore nella deregistrazione: %v\n", *serverID, err)
		}

		fmt.Printf("[%s] Shutdown completato\n", *serverID)
		os.Exit(0)
	}()

	// Avvia il server
	if err := grpcServer.Serve(listener); err != nil {
		fmt.Printf("Errore del server: %v\n", err)
		deregisterWithRegistry(*serverID, "localhost", int32(port))
	}
}

// registra il server nel service registry
func registerWithRegistry(serverID, address string, port int32) error {
	registryAddr := getRegistryAddress()
	conn, err := grpc.Dial(
		registryAddr,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("errore di connessione al registry: %v", err)
	}
	defer conn.Close()

	client := pb.NewServiceRegistryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Register(ctx, &pb.RegisterRequest{
		ServiceName: "CounterService",
		Address:     address,
		Port:        port,
	})

	if err != nil {
		return fmt.Errorf("errore nella registrazione: %v", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("registrazione fallita: %s", resp.GetMessage())
	}

	fmt.Printf("[%s] %s\n", serverID, resp.GetMessage())
	return nil
}

// deregistra il server dal service registry
func deregisterWithRegistry(serverID, address string, port int32) error {
	registryAddr := getRegistryAddress()
	conn, err := grpc.Dial(
		registryAddr,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("errore di connessione al registry: %v", err)
	}
	defer conn.Close()

	client := pb.NewServiceRegistryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Deregister(ctx, &pb.DeregisterRequest{
		ServiceName: "CounterService",
		Address:     address,
		Port:        port,
	})

	if err != nil {
		return fmt.Errorf("errore nella deregistrazione: %v", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("deregistrazione fallita: %s", resp.GetMessage())
	}

	fmt.Printf("[%s] %s\n", serverID, resp.GetMessage())
	return nil
}

// scopre gli altri server e li registra come peer nel counter server
func discoverAndRegisterPeers(serverID string, counterServer *server.CounterServer) error {
	registryAddr := getRegistryAddress()
	conn, err := grpc.Dial(
		registryAddr,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("errore di connessione al registry: %v", err)
	}
	defer conn.Close()

	client := pb.NewServiceRegistryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetServices(ctx, &pb.GetServicesRequest{
		ServiceName: "CounterService",
	})

	if err != nil {
		return fmt.Errorf("errore nella scoperta dei servizi: %v", err)
	}

	instances := resp.GetInstances()
	if len(instances) < 2 {
		return fmt.Errorf("nessun peer trovato (istanze: %d)", len(instances))
	}

	// Registra tutti gli altri server come peer
	serverHost := getServerHost()
	for _, instance := range instances {
		addr := fmt.Sprintf("%s:%d", instance.GetAddress(), instance.GetPort())
		// Non registra se stesso (confronta l'hostname)
		if instance.GetAddress() != serverHost {
			peerID := fmt.Sprintf("server-%s", instance.GetAddress())
			counterServer.RegisterPeer(peerID, addr)
			fmt.Printf("[%s] Peer registrato: %s @ %s\n", serverID, peerID, addr)
		}
	}

	fmt.Printf("[%s] Scoperte %d istanze del servizio Counter\n", serverID, len(instances))
	return nil
}
