package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type urlData struct {
	Paths map[string]string `json:"paths"`
}

type ShortenerService struct {
	mu       sync.RWMutex
	urlMap   map[string]string
	filePath string
}

func NewShortenerService(jsonPath string) (*ShortenerService, error) {
	service := &ShortenerService{
		urlMap:   make(map[string]string),
		filePath: jsonPath,
	}

	fileBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return service, nil
		}
		return nil, fmt.Errorf("error reading file %s: %w", jsonPath, err)
	}

	var data urlData
	if err := json.Unmarshal(fileBytes, &data); err != nil {
		return nil, fmt.Errorf("error JSON parsing %s: %w", jsonPath, err)
	}

	service.urlMap = data.Paths

	return service, nil
}

func (s *ShortenerService) GetURL(path string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	longURL, found := s.urlMap[path]
	return longURL, found
}

func (s *ShortenerService) AddURL(path, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("short url must start with '/'")
	}

	s.urlMap[path] = url

	return s.saveToFile()
}

func (s *ShortenerService) saveToFile() error {
	data := urlData{Paths: s.urlMap}

	fileBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error with JSON: %w", err)
	}

	err = os.WriteFile(s.filePath, fileBytes, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file %s: %w", s.filePath, err)
	}

	return nil
}
