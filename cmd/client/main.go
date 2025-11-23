package main

import (
	"context"
	"flag"
	"log"
	"time"

	pb "github.com/tu-usuario/proyecto-sd/api/reviews"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Definición de flags para configurar el comportamiento del cliente
	coordinatorAddr := flag.String("addr", "localhost:50051", "Dirección del Coordinador")
	clientID := flag.String("id", "cliente-1", "ID único del cliente")
	mode := flag.String("mode", "writer", "Modo de operación: 'writer' (RYW) o 'reader' (Monotonic)")
	flag.Parse()

	// Conexión gRPC con el Coordinador
	conn, err := grpc.Dial(*coordinatorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("No se pudo conectar al coordinador: %v", err)
	}
	defer conn.Close()

	client := pb.NewReviewServiceClient(conn)
	ctx := context.Background()

	// Lógica según el modo seleccionado
	if *mode == "writer" {
		runReadYourWrites(ctx, client, *clientID)
	} else if *mode == "reader" {
		runMonotonicReads(ctx, client, *clientID)
	} else {
		log.Fatalf("Modo desconocido: %s. Use 'writer' o 'reader'", *mode)
	}
}

// runReadYourWrites implementa el escenario: Escribir -> Leer inmediatamente (RYW)
func runReadYourWrites(ctx context.Context, client pb.ReviewServiceClient, id string) {
	log.Printf("--- Iniciando Cliente Escritor (%s) ---", id)
	
	reviewContent := "Esta es una reseña nueva escrita a las " + time.Now().Format(time.TimeOnly)

	// 1. Escritura
	log.Printf("Enviando escritura: '%s'", reviewContent)
	wResp, err := client.SubmitReview(ctx, &pb.WriteRequest{
		ClientId: id,
		Content:  reviewContent,
	})
	if err != nil {
		log.Fatalf("Error al escribir: %v", err)
	}
	log.Printf("Escritura confirmada. Estado: %s. Asignado a: %s", wResp.Status, wResp.AssignedNode)

	// Simular una pequeña pausa humana o cambio de página instantáneo
	time.Sleep(100 * time.Millisecond)

	// 2. Lectura Inmediata (Read Your Writes)
	// Como usamos el mismo ClientId, el Coordinador debería dirigirnos al nodo donde acabamos de escribir
	log.Printf("Solicitando lectura inmediata (verificando RYW)...")
	rResp, err := client.GetReview(ctx, &pb.ReadRequest{
		ClientId:    id,
		ReviewId:    "review-1", // ID fijo usado en el ejemplo
		IsMonotonic: false,      // No es necesario activar monotonic si confiamos en la sesión RYW del coordinador
	})

	if err != nil {
		log.Printf("Error al leer: %v", err)
	} else {
		log.Printf("Lectura exitosa: '%s'", rResp.Content)
		if rResp.Content == reviewContent {
			log.Println("✅ PRUEBA EXITOSA: Read Your Writes se cumplió.")
		} else {
			log.Println("❌ FALLO: El contenido leído no coincide con el escrito recientemente.")
		}
	}
}

// runMonotonicReads implementa el escenario: Leer repetidamente asegurando que el tiempo no retroceda
func runMonotonicReads(ctx context.Context, client pb.ReviewServiceClient, id string) {
	log.Printf("--- Iniciando Cliente Lector Monotónico (%s) ---", id)

	var lastClock *pb.VectorClock // Almacena el reloj de la última lectura exitosa

	for i := 1; i <= 5; i++ {
		log.Printf("\nIntento de lectura #%d...", i)

		req := &pb.ReadRequest{
			ClientId:       id,
			ReviewId:       "review-1",
			IsMonotonic:    true,      // Activa lógica monotónica en Coordinador
			LastKnownClock: lastClock, // Enviamos lo que sabemos del tiempo
		}

		resp, err := client.GetReview(ctx, req)
		if err != nil {
			log.Printf("Error o dato no encontrado aún: %v", err)
		} else {
			log.Printf("Recibido: '%s' desde nodo %s", resp.Content, resp.SourceNode)
			log.Printf("Reloj Vectorial: %v", resp.Clock)
			
			// Actualizamos nuestro conocimiento del tiempo
			lastClock = resp.Clock
		}

		// Esperar antes de la siguiente lectura para dar tiempo a que ocurran replicaciones
		time.Sleep(2 * time.Second)
	}
}