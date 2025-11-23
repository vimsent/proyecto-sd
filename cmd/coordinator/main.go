package main

import (
	"context"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/vimsent/proyecto-sd/api/reviews"
	"github.com/vimsent/proyecto-sd/pkg/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type coordinatorServer struct {
	pb.UnimplementedReviewServiceServer
	datanodes    []pb.ReviewServiceClient // Clientes gRPC hacia Datanodes
	clientMap    map[string]int           // RYW: Mapea ClientID -> Índice Datanode
	mu           sync.Mutex
}

// SubmitReview (RYW): Recibe escritura, asigna nodo y "pega" al cliente a ese nodo
func (s *coordinatorServer) SubmitReview(ctx context.Context, req *pb.WriteRequest) (*pb.WriteResponse, error) {
	s.mu.Lock()
	// Selección de nodo: Si el cliente ya escribió, usa el mismo (RYW), si no, Random o Round Robin
	nodeIndex, exists := s.clientMap[req.ClientId]
	if !exists {
		nodeIndex = rand.Intn(len(s.datanodes))
		s.clientMap[req.ClientId] = nodeIndex
	}
	s.mu.Unlock()

	// Crear Reloj Inicial
	clock := &pb.VectorClock{Versions: map[string]int64{}} 
	
	// Enviar al nodo primario elegido
	_, err := s.datanodes[nodeIndex].StoreReview(ctx, &pb.ReplicateRequest{
		ReviewId:     "review-1", // Hardcoded para ejemplo simple
		Content:      req.Content,
		Clock:        clock,
		SenderNodeId: "coordinator",
	})

	if err != nil {
		return nil, err
	}

	// Replicación Asíncrona (Simulando consistencia eventual)
	go s.replicateToOthers(nodeIndex, "review-1", req.Content, clock)

	return &pb.WriteResponse{Status: "Success", AssignedNode: fmt.Sprintf("Node-%d", nodeIndex)}, nil
}

// GetReview: Implementa RYW y Monotonic Reads
func (s *coordinatorServer) GetReview(ctx context.Context, req *pb.ReadRequest) (*pb.ReadResponse, error) {
	// 1. Lógica Read Your Writes [cite: 16, 67]
	targetNode := -1
	s.mu.Lock()
	if idx, ok := s.clientMap[req.ClientId]; ok {
		targetNode = idx
	}
	s.mu.Unlock()

	// Si no hay mapping previo y no es monotonic estricto, elige cualquiera
	if targetNode == -1 {
		targetNode = rand.Intn(len(s.datanodes))
	}

	// Intentar leer del nodo asignado
	resp, err := s.datanodes[targetNode].FetchReview(ctx, req)
	
	// 2. Lógica Monotonic Reads [cite: 14, 83]
	// Si el nodo responde con datos viejos (comparado con lo que sabe el cliente), buscar en otros
	if req.IsMonotonic && err == nil {
		if !domain.IsAfterOrEqual(resp.Clock, req.LastKnownClock) {
			log.Println("Consistencia Monotónica: Nodo desactualizado, buscando réplica más fresca...")
			// Buscar en otros nodos
			for i, node := range s.datanodes {
				if i == targetNode { continue }
				altResp, altErr := node.FetchReview(ctx, req)
				if altErr == nil && domain.IsAfterOrEqual(altResp.Clock, req.LastKnownClock) {
					return altResp, nil
				}
			}
		}
	}
	
	return resp, err
}

func (s *coordinatorServer) replicateToOthers(sourceIdx int, id, content string, clock *pb.VectorClock) {
	time.Sleep(2 * time.Second) // Simular latencia de red
	for i, node := range s.datanodes {
		if i == sourceIdx { continue }
		node.StoreReview(context.Background(), &pb.ReplicateRequest{
			ReviewId: id, Content: content, Clock: clock,
		})
	}
}

func main() {
	// Configuración de conexiones a Datanodes (IPs desde ENV)
	dnAddrs := strings.Split(os.Getenv("DATANODE_ADDRS"), ",")
	var clients []pb.ReviewServiceClient

	for _, addr := range dnAddrs {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil { log.Fatal(err) }
		clients = append(clients, pb.NewReviewServiceClient(conn))
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil { log.Fatal(err) }

	s := grpc.NewServer()
	pb.RegisterReviewServiceServer(s, &coordinatorServer{
		datanodes: clients,
		clientMap: make(map[string]int),
	})

	log.Println("Coordinador escuchando en :50051")
	if err := s.Serve(lis); err != nil { log.Fatal(err) }
}