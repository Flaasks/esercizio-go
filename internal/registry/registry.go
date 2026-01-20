package registry

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/Flaasks/esercizio-go/pkg/pb"
)

// Gestisce la registrazione dei servizi
type ServiceRegistry struct {
	pb.UnimplementedServiceRegistryServer

	// Map di servizi: service_name -> lista di istanze
	services map[string][]*pb.ServiceInstance
	mu       sync.RWMutex
}

// Crea una nuova istanza di ServiceRegistry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string][]*pb.ServiceInstance),
	}
}

// Registra un nuovo servizio
func (sr *ServiceRegistry) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	serviceName := req.GetServiceName()
	instance := &pb.ServiceInstance{
		Address: req.GetAddress(),
		Port:    req.GetPort(),
	}

	// Verifica se il servizio esiste già
	if _, exists := sr.services[serviceName]; !exists {
		sr.services[serviceName] = make([]*pb.ServiceInstance, 0)
	}

	// Verifica che l'istanza non sia già registrata
	for _, inst := range sr.services[serviceName] {
		if inst.Address == instance.Address && inst.Port == instance.Port {
			return &pb.RegisterResponse{
				Success: false,
				Message: fmt.Sprintf("Servizio %s già registrato su %s:%d", serviceName, instance.Address, instance.Port),
			}, nil
		}
	}

	// Aggiungi la nuova istanza
	sr.services[serviceName] = append(sr.services[serviceName], instance)

	msg := fmt.Sprintf("Servizio %s registrato su %s:%d", serviceName, instance.Address, instance.Port)
	fmt.Printf("[REGISTRY] %s\n", msg)

	return &pb.RegisterResponse{
		Success: true,
		Message: msg,
	}, nil
}

// Deregistra un servizio registrato
func (sr *ServiceRegistry) Deregister(ctx context.Context, req *pb.DeregisterRequest) (*pb.DeregisterResponse, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	serviceName := req.GetServiceName()
	instances, exists := sr.services[serviceName]

	if !exists {
		return &pb.DeregisterResponse{
			Success: false,
			Message: fmt.Sprintf("Servizio %s non trovato", serviceName),
		}, nil
	}

	// Filtra per rimuovere l'istanza
	var newInstances []*pb.ServiceInstance
	found := false

	for _, inst := range instances {
		if inst.Address == req.GetAddress() && inst.Port == req.GetPort() {
			found = true
		} else {
			newInstances = append(newInstances, inst)
		}
	}

	if !found {
		return &pb.DeregisterResponse{
			Success: false,
			Message: fmt.Sprintf("Istanza %s:%d non registrata", req.GetAddress(), req.GetPort()),
		}, nil
	}

	// Aggiorna o rimuovi il servizio
	if len(newInstances) == 0 {
		delete(sr.services, serviceName)
	} else {
		sr.services[serviceName] = newInstances
	}

	msg := fmt.Sprintf("Servizio %s deregistrato da %s:%d", serviceName, req.GetAddress(), req.GetPort())
	fmt.Printf("[REGISTRY] %s\n", msg)

	return &pb.DeregisterResponse{
		Success: true,
		Message: msg,
	}, nil
}

// Restituisce la lista di istanze per un servizio
func (sr *ServiceRegistry) GetServices(ctx context.Context, req *pb.GetServicesRequest) (*pb.GetServicesResponse, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	serviceName := req.GetServiceName()
	instances, exists := sr.services[serviceName]

	if !exists || len(instances) == 0 {
		fmt.Printf("[REGISTRY] Richiesta per servizio %s - nessuna istanza trovata\n", serviceName)
		return &pb.GetServicesResponse{
			Instances: make([]*pb.ServiceInstance, 0),
		}, nil
	}

	fmt.Printf("[REGISTRY] Richiesta per servizio %s - trovate %d istanze\n", serviceName, len(instances))

	return &pb.GetServicesResponse{
		Instances: instances,
	}, nil
}
