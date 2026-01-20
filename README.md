

Architettura

Questa applicazione distribuita implementa:
- Un Service Registry centralizzato per la gestione dei servizi
- 2 Server replicati con un servizio Counter (stateful) condiviso
- Un Client con client-side service discovery e 2 algoritmi di load balancing
- Registrazione automatica dei server
- Shutdown graceful con cleanup (i server si deregistrano automaticamente dal registry)
- Containerizzazione completa con Docker usando l'immagine ufficiale Go


Parte Opzionale
- Due algoritmi di load balancing
  - Round-Robin
  - Random
- Containerizzazione Docker
  - Dockerfile multi-stage per ogni componente
  - Docker Compose per orchestrazione
  - Immagine ufficiale Go (golang:1.21-alpine)


Requisiti
- Go 1.24.0 o superiore
- Protocol Buffer compiler (protoc)
- Docker Desktop installato
- Docker Compose v2.0+


Compilazione

```bash
go build -o bin/registry ./cmd/registry
go build -o bin/server ./cmd/server
go build -o bin/client ./cmd/client
```

Avvio con Docker

```bash
# Build e avvio di registry + 2 server
docker-compose up -d --build

# Avvio del client Round-Robin 
docker-compose --profile client run -it client-roundrobin

# Se il client è già in esecuzione
docker exec -it counter-client-rr /root/client -v -lb round-robin

# Avvia il client con Load Balancing Random
docker-compose --profile client run -it client-random

# Oppure con i container già avviati
docker exec -it counter-client-random /root/client -v -lb random

# Stop e cleanup dei container
docker-compose down -v
```


Comandi del Client:

- **inc [amount]** - Incrementa il contatore (default: 1)
  ```
  inc
  inc 5
  ```

- **dec [amount]** - Decrementa il contatore (default: 1)
  ```
  dec
  dec 3
  ```

- **get** - Ottiene il valore attuale del contatore
  ```
  get
   [localhost:6000] Valore attuale: 2
  ```

- **list** - Visualizza i server nella cache

- **exit** - Esce dal client



Load Balancing: Due Algoritmi

Il client supporta due algoritmi di load balancing:

1. Round-Robin (Default)
Distribuisce le richieste in modo circolare e deterministico:
- Prima richiesta → Server 1 (6000)
- Seconda richiesta → Server 2 (6001)
- Terza richiesta → Server 1 (6000)
- E così via...

Vantaggi: Distribuzione equa e prevedibile  

2. Random
Seleziona casualmente un server per ogni richiesta:
- Ogni richiesta sceglie random tra i server disponibili
- Distribuzione statistica uniforme nel lungo periodo
- Non deterministico

Vantaggi: Semplice, nessuno stato da mantenere  



La lista dei server viene cachata nella sessione del client per evitare multiple richieste al registry



Implementazione dettagliata:

Service Registry
- Gestisce una mappa in-memory di servizi
- Thread-safe con mutex RWLock
- API gRPC per Register, Deregister, GetServices

Counter Server
- Servizio stateful che mantiene un contatore intero
- Supporta Increment, Decrement, GetValue
- Ogni server ha un ID univoco per identificarlo

Client
- Interroga il registry in startup
- Cacha la lista dei server per la sessione
- Implementa load balancing 










