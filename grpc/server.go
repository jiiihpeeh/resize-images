package grpc

import (
	"image-resizer/handlers"
	"image-resizer/pb"
	"io"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedResizerServer
	imageHandler *handlers.ImageHandler
}

func NewServer(h *handlers.ImageHandler) *Server {
	return &Server{imageHandler: h}
}

func (s *Server) Resize(stream pb.Resizer_ResizeServer) error {
	// 1. Read Metadata (first message)
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	meta := req.GetMetadata()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "first message must be metadata")
	}

	// 2. Create temp file for processing
	tmpFile, err := os.CreateTemp("", "grpc-image-*.img")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create temp file: %v", err)
	}
	tempPath := tmpFile.Name()
	defer os.Remove(tempPath)

	// 3. Read chunks and write to temp file
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			tmpFile.Close()
			return err
		}
		chunk := req.GetChunk()
		if chunk != nil {
			if _, err := tmpFile.Write(chunk); err != nil {
				tmpFile.Close()
				return status.Errorf(codes.Internal, "failed to write image data: %v", err)
			}
		}
	}
	tmpFile.Close()

	// Convert pb tasks to internal jobs
	var jobs []handlers.ResizeBatchJob
	for _, t := range meta.Tasks {
		jobs = append(jobs, handlers.ResizeBatchJob{
			Key:      t.Key,
			Width:    int(t.Width),
			Height:   int(t.Height),
			Formats:  t.Formats,
			Quality:  int(t.Quality),
			Lossless: t.Lossless,
		})
	}

	if len(jobs) == 0 {
		return status.Error(codes.InvalidArgument, "no tasks provided")
	}

	// 5. Prepare response writer
	responseWriter := &StreamResponseWriter{stream: stream}

	// 6. Process
	// Determine if single or batch
	if len(jobs) == 1 && meta.ArchiveType == "" {
		buf, contentType, err := s.imageHandler.ProcessSingle(tempPath, jobs[0])
		if err != nil {
			return status.Errorf(codes.Internal, "processing failed: %v", err)
		}

		// Send metadata
		if err := stream.Send(&pb.ResizeResponse{
			Payload: &pb.ResizeResponse_Metadata{
				Metadata: &pb.ResponseMetadata{
					ContentType: contentType,
					Filename:    "image." + jobs[0].Formats[0],
				},
			},
		}); err != nil {
			return err
		}

		// Send data
		if _, err := responseWriter.Write(buf); err != nil {
			return err
		}
		return nil
	}

	// Batch processing
	archiveType := meta.ArchiveType
	if archiveType == "" {
		archiveType = "tar.gz"
	}

	filename := "images.tar.gz"
	contentType := "application/x-tar+gzip"
	if archiveType == "zip" {
		filename = "images.zip"
		contentType = "application/zip"
	} else if archiveType == "tar" {
		filename = "images.tar"
		contentType = "application/x-tar"
	}

	// Send metadata
	if err := stream.Send(&pb.ResizeResponse{
		Payload: &pb.ResizeResponse_Metadata{
			Metadata: &pb.ResponseMetadata{
				ContentType: contentType,
				Filename:    filename,
			},
		},
	}); err != nil {
		return err
	}

	// Process and stream data
	if err := s.imageHandler.ProcessBatch(responseWriter, tempPath, jobs, archiveType, "image"); err != nil {
		return status.Errorf(codes.Internal, "batch processing failed: %v", err)
	}

	return nil
}

type StreamResponseWriter struct {
	stream pb.Resizer_ResizeServer
}

func (w *StreamResponseWriter) Write(p []byte) (n int, err error) {
	const chunkSize = 64 * 1024 // 64KB chunks
	total := len(p)
	for len(p) > 0 {
		n := chunkSize
		if n > len(p) {
			n = len(p)
		}
		if err := w.stream.Send(&pb.ResizeResponse{
			Payload: &pb.ResizeResponse_Chunk{
				Chunk: p[:n],
			},
		}); err != nil {
			return 0, err
		}
		p = p[n:]
	}
	return total, nil
}
