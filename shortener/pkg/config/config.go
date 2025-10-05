package config

import (
	"flag"
	"log"
)

type Config struct {
	ListenAddr string
	UrlsPath   string
}

func New() *Config {
	var cfg Config

	flag.StringVar(&cfg.ListenAddr, "addr", ":8080", "Address for the server to listen on")
	flag.StringVar(&cfg.UrlsPath, "f", "urls.json", "Path to the JSON file with URL mappings")

	flag.Parse()

	log.Printf("Loaded config: Address [%s], URL-file path [%s]", cfg.ListenAddr, cfg.UrlsPath)

	return &cfg
}
