package main

import (
	"context"
	"flag"
	"fmt"
	"image-resizer/pb"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	serverAddr := flag.String("addr", "localhost:50051", "The server address in the format of host:port")
	imagePath := flag.String("image", "cafe.jpg", "Path to the image file")
	flag.Parse()

	// 1. Connect to gRPC server
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewResizerClient(conn)

	// 2. Open image file
	file, err := os.Open(*imagePath)
	if err != nil {
		log.Fatalf("failed to open image: %v", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := client.Resize(ctx)
	if err != nil {
		log.Fatalf("error creating stream: %v", err)
	}

	// 3. Send Metadata (First message)
	err = stream.Send(&pb.ResizeRequest{
		Payload: &pb.ResizeRequest_Metadata{
			Metadata: &pb.ResizeMetadata{
				Tasks: []*pb.Task{
					{Key: "thumb", Width: 200, Formats: []string{"jpg"}},
					{Key: "large", Width: 800, Formats: []string{"webp"}, Quality: 80},
				},
				ArchiveType: "tar.gz",
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to send metadata: %v", err)
	}

	// 4. Stream Image Data
	buf := make([]byte, 32*1024) // 32KB chunks
	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}

		err = stream.Send(&pb.ResizeRequest{
			Payload: &pb.ResizeRequest_Chunk{
				Chunk: buf[:n],
			},
		})
		if err != nil {
			log.Fatalf("failed to send chunk: %v", err)
		}
	}

	// Close send direction to signal end of upload
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("failed to close stream: %v", err)
	}

	// 5. Receive Response
	var outFile *os.File
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("failed to receive: %v", err)
		}

		if meta := res.GetMetadata(); meta != nil {
			fmt.Printf("Receiving %s (%s)...\n", meta.Filename, meta.ContentType)
			outFile, err = os.Create(meta.Filename)
			if err != nil {
				log.Fatalf("failed to create output file: %v", err)
			}
			defer outFile.Close()
		} else if chunk := res.GetChunk(); chunk != nil {
			if outFile == nil {
				log.Fatal("received chunk before metadata")
			}
			if _, err := outFile.Write(chunk); err != nil {
				log.Fatalf("failed to write chunk: %v", err)
			}
		}
	}
	fmt.Println("Done.")
}
