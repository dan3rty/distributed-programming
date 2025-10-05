package transport

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

type URLService interface {
	GetURL(path string) (string, bool)
	AddURL(path, url string) error
}

type Handler struct {
	service URLService
	router  *mux.Router
}

func NewHandler(service URLService) *Handler {
	h := &Handler{
		service: service,
		router:  mux.NewRouter(),
	}

	h.router.HandleFunc("/admin", h.serveAdminPage).Methods(http.MethodGet)
	h.router.HandleFunc("/add", h.addURLHandler).Methods(http.MethodPost)
	h.router.HandleFunc("/{shortPath}", h.redirectHandler).Methods(http.MethodGet)
	h.router.NotFoundHandler = http.HandlerFunc(h.fallbackHandler)

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *Handler) redirectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortPath := "/" + vars["shortPath"]

	log.Printf("Request to short URL: %s", shortPath)

	longURL, found := h.service.GetURL(shortPath)
	if !found {
		log.Printf("Short URL not found: %s", shortPath)
		h.fallbackHandler(w, r)
		return
	}

	log.Printf("Redirect from %s to %s", shortPath, longURL)
	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}

func (h *Handler) serveAdminPage(w http.ResponseWriter, _ *http.Request) {
	tmplPath := filepath.Join("web", "templates", "index.html")

	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("Error template parsing: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Printf("Error with template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) addURLHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Incorrect data", http.StatusBadRequest)
		return
	}

	shortPath := r.FormValue("short_path")
	longURL := r.FormValue("long_url")

	if shortPath == "" || longURL == "" {
		http.Error(w, "All fields must be not empty", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(shortPath, "/") {
		shortPath = "/" + shortPath
	}

	log.Println(h.service)
	err := h.service.AddURL(shortPath, longURL)
	if err != nil {
		log.Printf("Error adding url: %v", err)
		http.Error(w, "Error while add new URL", http.StatusInternalServerError)
		return
	}

	log.Printf("New URL added: %s -> %s", shortPath, longURL)

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *Handler) fallbackHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	_, err := fmt.Fprintf(w, "<h1>Short URL not found</h1>")
	if err != nil {
		return
	}
}
