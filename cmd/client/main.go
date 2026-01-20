package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"

	"github.com/Flaasks/esercizio-go/internal/client"
	pb "github.com/Flaasks/esercizio-go/pkg/pb"
)

var (
	verbose   = flag.Bool("v", false, "Modalità verbose")
	algorithm = flag.String("lb", "round-robin", "Algoritmo di load balancing: 'round-robin' o 'random'")
)

func getRegistryAddress() string {
	if addr := os.Getenv("REGISTRY_ADDRESS"); addr != "" {
		return addr
	}
	return "localhost:5000"
}

func main() {
	flag.Parse()

	fmt.Println("Client Counter con Load Balancing avviato")

	// Determina l'algoritmo di load balancing
	var lbAlgorithm client.LoadBalancingAlgorithm
	var algorithmName string

	switch *algorithm {
	case "random":
		lbAlgorithm = client.Random
		algorithmName = "Random"
	case "round-robin":
		lbAlgorithm = client.RoundRobin
		algorithmName = "Round-Robin"
	default:
		fmt.Printf("Algoritmo '%s' non valido, uso Round-Robin\n", *algorithm)
		lbAlgorithm = client.RoundRobin
		algorithmName = "Round-Robin"
	}

	fmt.Printf("Algoritmo Load Balancing: %s\n\n", algorithmName)

	// Crea il load balancer con l'algoritmo selezionato
	lb := client.NewLoadBalancer(lbAlgorithm)

	// Interroga il registry e ottiene la lista dei server
	fmt.Println("[CLIENT] Interrogando il Service Registry...")
	servers, err := getServersFromRegistry()
	if err != nil {
		fmt.Printf("Errore nella lettura dal registry: %v\n", err)
		return
	}

	if len(servers) == 0 {
		fmt.Println("Nessun server disponibile!")
		return
	}

	// Converte i server per il load balancer
	serverInfos := make([]*client.ServerInfo, len(servers))
	for i, s := range servers {
		serverInfos[i] = &client.ServerInfo{
			Address: s.GetAddress(),
			Port:    s.GetPort(),
		}
		if *verbose {
			fmt.Printf("  - %s:%d\n", s.GetAddress(), s.GetPort())
		}
	}

	lb.SetServers(serverInfos)

	fmt.Printf("\nTrovati: %d server disponibili\n", lb.GetServersCount())
	fmt.Println("Cache della lista server nella sessione")
	fmt.Println("\nComandi disponibili:")
	fmt.Println("  inc <amount>  - Incrementa il contatore (quorum write)")
	fmt.Println("  dec <amount>  - Decrementa il contatore (quorum write)")
	fmt.Println("  get           - Ottieni il valore attuale")
	fmt.Println("  list          - Visualizza i server")
	fmt.Println("  exit          - Esci dal client")
	fmt.Println()

	// Loop interattivo
	reader := bufio.NewReader(os.Stdin)

	for {
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]

		switch command {
		case "inc":
			amount := int32(1)
			if len(parts) > 1 {
				if a, err := strconv.Atoi(parts[1]); err == nil {
					amount = int32(a)
				}
			}
			handleIncrement(lb, amount)

		case "dec":
			amount := int32(1)
			if len(parts) > 1 {
				if a, err := strconv.Atoi(parts[1]); err == nil {
					amount = int32(a)
				}
			}
			handleDecrement(lb, amount)

		case "get":
			handleGetValue(lb)

		case "list":
			fmt.Printf("\nServer disponibili nella cache:\n")
			for i := 0; i < lb.GetServersCount(); i++ {
				si := lb.GetNextServer()
				fmt.Printf("  %d. %s:%d\n", i+1, si.Address, si.Port)
			}
			fmt.Println()

		case "exit":
			fmt.Println("Arrivederci!")
			return

		default:
			fmt.Println("Comando sconosciuto. Usa 'inc', 'dec', 'get', 'list' o 'exit'")
		}
	}
}

func handleIncrement(lb *client.LoadBalancer, amount int32) {
	// invia ad un solo server che replicherà agli altri
	server := lb.GetNextServer()
	if server == nil {
		fmt.Println("Nessun server disponibile!")
		return
	}

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", server.Address, server.Port),
		grpc.WithInsecure(),
	)
	if err != nil {
		fmt.Printf("Errore connessione a %s:%d: %v\n", server.Address, server.Port, err)
		return
	}
	defer conn.Close()

	client := pb.NewCounterServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Increment(ctx, &pb.IncrementRequest{
		Amount: amount,
	})

	if err != nil {
		fmt.Printf("Errore increment: %v\n", err)
		return
	}

	fmt.Printf(" [%s:%d] Incremento di %d -> Valore: %d (versione: %d)\n",
		server.Address, server.Port, amount, resp.GetValue(), resp.GetVersion())

	if *verbose {
		fmt.Printf("  [REPLICAZIONE] Il server replicherà l'operazione agli altri %d peer\n",
			lb.GetServersCount()-1)
	}
}

func handleDecrement(lb *client.LoadBalancer, amount int32) {
	// invia ad un solo server che replicherà agli altri
	server := lb.GetNextServer()
	if server == nil {
		fmt.Println("Nessun server disponibile!")
		return
	}

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", server.Address, server.Port),
		grpc.WithInsecure(),
	)
	if err != nil {
		fmt.Printf("Errore connessione a %s:%d: %v\n", server.Address, server.Port, err)
		return
	}
	defer conn.Close()

	client := pb.NewCounterServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Decrement(ctx, &pb.DecrementRequest{
		Amount: amount,
	})

	if err != nil {
		fmt.Printf("Errore decrement: %v\n", err)
		return
	}

	fmt.Printf(" [%s:%d] Decremento di %d -> Valore: %d (versione: %d)\n",
		server.Address, server.Port, amount, resp.GetValue(), resp.GetVersion())

	if *verbose {
		fmt.Printf("  [REPLICAZIONE] Il server replicherà l'operazione agli altri %d peer\n",
			lb.GetServersCount()-1)
	}
}

func handleGetValue(lb *client.LoadBalancer) {
	// legge da un server (any-read è sufficiente se quorum-write è garantito)
	server := lb.GetNextServer()
	if server == nil {
		fmt.Println("Nessun server disponibile!")
		return
	}

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", server.Address, server.Port),
		grpc.WithInsecure(),
	)
	if err != nil {
		fmt.Printf("Errore di connessione a %s:%d: %v\n", server.Address, server.Port, err)
		return
	}
	defer conn.Close()

	client := pb.NewCounterServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetValue(ctx, &pb.GetValueRequest{})

	if err != nil {
		fmt.Printf("Errore nella lettura del valore: %v\n", err)
		return
	}

	fmt.Printf(" [%s:%d] Valore attuale: %d (versione: %d)\n",
		server.Address, server.Port, resp.GetValue(), resp.GetVersion())
	if *verbose {
		fmt.Printf("  Server ID: %s\n", resp.GetServerId())
	}
}

func getServersFromRegistry() ([]*pb.ServiceInstance, error) {
	registryAddr := getRegistryAddress()
	conn, err := grpc.Dial(
		registryAddr,
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("errore di connessione al registry: %v", err)
	}
	defer conn.Close()

	client := pb.NewServiceRegistryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetServices(ctx, &pb.GetServicesRequest{
		ServiceName: "CounterService",
	})

	if err != nil {
		return nil, fmt.Errorf("errore nella richiesta al registry: %v", err)
	}

	return resp.GetInstances(), nil
}
