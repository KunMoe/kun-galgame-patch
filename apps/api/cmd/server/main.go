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

	// The container healthcheck runs this binary every 10s; probing before Load
	// keeps it from opening a database connection to read the config store.
	health.MaybeProbe(config.Env("KUN_SERVER_PORT", "5214"), "/healthz")

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

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
