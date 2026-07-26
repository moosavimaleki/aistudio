package main

import (
	"github.com/hamed/aistudio-api/internal/browserinterface"
	"github.com/hamed/aistudio-api/internal/metrics"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	config, err := browserinterface.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	broker := browserinterface.NewBroker()
	metricStore, err := metrics.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	defer metricStore.Close()
	fleet := browserinterface.NewFleet(broker, config)
	if err := fleet.Start(); err != nil {
		log.Fatal(err)
	}
	defer fleet.Close()
	go fleet.Warm()
	port := os.Getenv("PORT")
	if port == "" {
		port = "3345"
	}
	handler := metrics.HTTP(metricStore, func(path string) bool { return path == "/health" || strings.HasPrefix(path, "/internal/jobs") }, browserinterface.NewServer(fleet, broker).Handler())
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("browser interface listening on %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}
