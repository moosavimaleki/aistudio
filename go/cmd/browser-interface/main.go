package main

import (
	"github.com/hamed/aistudio-api/go/browserinterface"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	config, err := browserinterface.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	broker := browserinterface.NewBroker()
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
	server := &http.Server{Addr: ":" + port, Handler: browserinterface.NewServer(fleet, broker).Handler(), ReadHeaderTimeout: 10 * time.Second}
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
