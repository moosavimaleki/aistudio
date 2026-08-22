package main

import (
	"github.com/hamed/aistudio-api/internal/aistudio"
	"github.com/hamed/aistudio-api/internal/gencontent"
	"github.com/hamed/aistudio-api/internal/metrics"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	settings, err := aistudio.LoadSettings("")
	if err != nil {
		log.Fatal(err)
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	pool, err := gencontent.NewPool(redisURL, intValue("TAB_POOL_MAX", 100), time.Duration(intValue("TAB_POOL_WAIT_SECONDS", 5))*time.Second, time.Duration(intValue("TAB_POOL_LEASE_SECONDS", 600))*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	metricStore, err := metrics.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	defer metricStore.Close()
	port := os.Getenv("GENCONTENT_PORT")
	if port == "" {
		port = "8000"
	}
	dashboard := gencontent.NewDashboard(metricStore, pool, os.Getenv("FACTORY_ORIGIN"))
	serverHandler := gencontent.NewServer(gencontent.NewService(settings, pool), pool, dashboard).Handler()
	handler := metrics.HTTP(metricStore, func(path string) bool {
		return path == "/" || path == "/health" || strings.HasPrefix(path, "/dashboard")
	}, serverHandler)
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("gencontent listening on %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}
func intValue(name string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(name)); err == nil && value > 0 {
		return value
	}
	return fallback
}
