package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	pb "github.com/tu-usuario/proyecto-sd/api/reviews"
	"github.com/tu-usuario/proyecto-sd/pkg/domain"
	"google.golang.org/grpc"
)

type dataNodeServer struct {
	pb.UnimplementedReviewServiceServer
	mu      sync.Mutex
	storage map[string]*pb.Review // Almacenamiento en memoria
	nodeID  string
}

func (s *dataNodeServer) StoreReview(ctx context.Context, req *pb.ReplicateRequest) (*pb.ReplicateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.storage[req.ReviewId]
	
	// Lógica de Consistencia Eventual: Merge de relojes
	newClock := req.Clock
	if exists {
		newClock = domain.MergeClocks(current.Clock, req.Clock)
	}
	
	// Actualizar reloj local (evento de recibir mensaje)
	domain.IncrementClock(newClock, s.nodeID)

	s.storage[req.ReviewId] = &pb.Review{
		Id:      req.ReviewId,
		Content: req.Content,
		Clock:   newClock,
	}
	
	log.Printf("[%s] Datos guardados/actualizados: %v", s.nodeID, req.ReviewId)
	return &pb.ReplicateResponse{Success: true}, nil
}

func (s *dataNodeServer) FetchReview(ctx context.Context, req *pb.ReadRequest) (*pb.ReadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.storage[req.ReviewId]
	if !exists {
		return nil, fmt.Errorf("review not found")
	}

	return &pb.ReadResponse{
		Content:    data.Content,
		Clock:      data.Clock,
		SourceNode: s.nodeID,
	}, nil
}

func main() {
	nodeID := os.Getenv("NODE_ID")
	port := os.Getenv("PORT")

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Fallo al escuchar: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterReviewServiceServer(s, &dataNodeServer{
		storage: make(map[string]*pb.Review),
		nodeID:  nodeID,
	})

	log.Printf("Datanode %s escuchando en puerto %s", nodeID, port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Fallo al servir: %v", err)
	}
}