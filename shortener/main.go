package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"shortener/pkg/config"
	"shortener/pkg/service"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	httphandler "shortener/pkg/service/transport"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	file, err := os.OpenFile("shortener.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(file)
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {
				log.Fatal(err)
			}
		}(file)
	} else {
		log.Info("Cannot log to file, using default stderr")
	}

	cfg := config.New()
	shortenerService, err := service.NewShortenerService(cfg.UrlsPath)
	if err != nil {
		log.Fatalf("Cannot start server: %v", err)
	}
	handler := httphandler.NewHandler(shortenerService)

	log.WithFields(log.Fields{
		"address": cfg.ListenAddr,
	}).Info("Server starts...")

	srv := startServer(cfg.ListenAddr, handler)

	killSignalChan := getKillSignalChan()
	waitForKillSignalChan(killSignalChan)

	log.Info("Server stops...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Error while stopping server: %v", err)
	}

	log.Info("Server gracefully stopped.")
}

func startServer(serverUrl string, router http.Handler) *http.Server {
	srv := &http.Server{Addr: serverUrl, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Error listening port: %s\n", err)
		}
	}()

	return srv
}

func getKillSignalChan() chan os.Signal {
	osKillSignalChan := make(chan os.Signal, 1)
	signal.Notify(osKillSignalChan, os.Kill, os.Interrupt, syscall.SIGTERM)
	return osKillSignalChan
}

func waitForKillSignalChan(killSignalChan <-chan os.Signal) {
	killSignal := <-killSignalChan
	switch killSignal {
	case os.Interrupt:
		log.Info("Got SIGINT...")
	case syscall.SIGTERM:
		log.Info("Got SIGTERM...")
	}
}
