package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"kun-galgame-patch-api/internal/app"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/health"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	logger.Init(os.Getenv("KUN_SERVER_MODE"))
	cfg := config.Load()

	// `server healthcheck` (container HEALTHCHECK on distroless, which has no
	// shell): probe the already-running server's /api/v1/health and exit 0/1.
	// No-op for the normal `server` invocation. Runs before app.New so the
	// probe never touches the DB/Redis.
	health.MaybeProbe(cfg.Server.Port, "/api/v1/health")

	application := app.New(cfg)
	application.RegisterRoutes()

	go func() {
		addr := fmt.Sprintf(":%s", cfg.Server.Port)
		log.Printf("Server starting on %s", addr)
		if err := application.Fiber.Listen(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if err := application.Fiber.ShutdownWithContext(context.Background()); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
	if application.CronStop != nil {
		application.CronStop()
	}
	log.Println("Server exited")
}
