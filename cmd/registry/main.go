package main

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"google.golang.org/grpc"

	"github.com/Flaasks/esercizio-go/internal/registry"
	pb "github.com/Flaasks/esercizio-go/pkg/pb"
)

const defaultPort = 5000

func getPort() int {
	if portStr := os.Getenv("REGISTRY_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}
	return defaultPort
}

func main() {
	registryPort := getPort()

	// Crea il listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", registryPort))
	if err != nil {
		fmt.Printf("Errore nella creazione del listener: %v\n", err)
		return
	}
	defer listener.Close()

	// Crea il server gRPC
	grpcServer := grpc.NewServer()

	// Registra il servizio
	registrySvc := registry.NewServiceRegistry()
	pb.RegisterServiceRegistryServer(grpcServer, registrySvc)

	fmt.Printf("Service Registry avviato su localhost:%d\n", registryPort)
	fmt.Printf("In ascolto per le registrazioni di servizi...\n")

	// Avvia il server
	if err := grpcServer.Serve(listener); err != nil {
		fmt.Printf("Errore del server: %v\n", err)
	}
}
