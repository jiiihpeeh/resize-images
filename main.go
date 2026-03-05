package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"

	"google.golang.org/grpc"

	"image-resizer/config"
	grpc_server "image-resizer/grpc"
	"image-resizer/handlers"
	"image-resizer/middleware"
	"image-resizer/pb"
	"image-resizer/setup"
)

func main() {
	tuiMode := flag.Bool("tui", false, "Run first-time setup TUI")
	flag.Parse()

	if *tuiMode {
		setup.Run()
		return
	}

	// Set a soft memory limit of 3GB.
	debug.SetMemoryLimit(3 * 1024 * 1024 * 1024)

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it")
	}

	cfg := config.Load()

	// Log loaded configuration for verification
	log.Println("--------------------------------------------------")
	log.Printf("Server Configuration:")
	log.Printf("  Address: %s:%s", cfg.Server.Host, cfg.Server.Port)
	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin (default)"
	}
	log.Printf("  Admin User: %s", adminUser)
	regSecret := os.Getenv("REGISTRATION_SECRET")
	// Initialize the thread-safe secret manager
	handlers.InitRegistrationSecret(regSecret)

	if regSecret != "" {
		prefix := regSecret
		if len(regSecret) > 3 {
			prefix = regSecret[:3] + "..."
		}
		log.Printf("  Registration Secret: [CONFIGURED] (starts with %s)", prefix)
	} else {
		log.Println("  Registration Secret: [NOT CONFIGURED]")
	}
	log.Println("--------------------------------------------------")

	// Initialize SQLite database
	db, err := sql.Open("sqlite3", "./users.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	createTableSQL := `CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT,
		passkey_hash TEXT,
		blocked BOOLEAN DEFAULT 0
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	// Attempt to add passkey_hash column if it doesn't exist (for migration)
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN passkey_hash TEXT")

	createActivityTableSQL := `CREATE TABLE IF NOT EXISTS activity_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		action TEXT,
		details TEXT,
		ip_address TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(createActivityTableSQL); err != nil {
		log.Fatalf("Failed to create activity_logs table: %v", err)
	}

	app := fiber.New(fiber.Config{
		// Image uploads can be large, default is 4MB.
		BodyLimit:    50 * 1024 * 1024,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	imageHandler := handlers.NewImageHandler(cfg)
	authHandler := handlers.NewAuthHandler(cfg, db)
	regHandler := handlers.NewRegistrationHandler(cfg, db)

	concurrencyLimiter := middleware.NewConcurrencyLimiter(cfg.RateLimit.MaxConcurrent)
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit.RequestsPerMin, 60)
	activityLogger := middleware.NewActivityLogger(db)

	// Serve static files from web directory
	app.Static("/", "./web")

	app.Get("/health", imageHandler.Health)

	app.Post("/auth/register", rateLimiter.Handle(), regHandler.Register)
	app.Post("/auth/login", rateLimiter.Handle(), authHandler.Login)
	app.Post("/auth/refresh", rateLimiter.Handle(), authHandler.Refresh)

	protected := app.Group("", middleware.JWTAuthMiddleware(cfg.JWT.Secret))
	protected.Get("/resize", concurrencyLimiter.Handle(), rateLimiter.Handle(), activityLogger, imageHandler.Resize)
	protected.Post("/resize", concurrencyLimiter.Handle(), rateLimiter.Handle(), activityLogger, imageHandler.Resize)
	protected.Post("/auth/block", authHandler.BlockUser)

	go func() {
		addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start gRPC server
	go func() {
		grpcPort := os.Getenv("GRPC_PORT")
		if grpcPort == "" {
			grpcPort = "50051"
		}
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatalf("Failed to listen for gRPC: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterResizerServer(s, grpc_server.NewServer(imageHandler))
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	// Stop accepting new requests and wait for active ones to finish
	if err := app.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}
	// Cleanup resources after requests are drained
	imageHandler.Shutdown()
}
