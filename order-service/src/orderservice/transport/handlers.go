package transport

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

func Router() http.Handler {
	r := mux.NewRouter()
	s := r.PathPrefix("/api/v1").Subrouter()
	s.HandleFunc("/hello-world", helloWorld).Methods(http.MethodGet)
	s.HandleFunc("/kitty", getKitty).Methods(http.MethodGet)
	s.HandleFunc("/kitty/{id}", getKittyKitty).Methods(http.MethodGet)

	return logMiddleware(r)
}

type Kitty struct {
	Name string `json:"name"`
}

func getKittyKitty(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	some := r.URL.Query().Get("some")

	if _, err := io.WriteString(w, id+some); err != nil {
		log.WithField("err", err).Error("failed to write kitty kitty")
	}
}

func getKitty(w http.ResponseWriter, _ *http.Request) {
	cat := Kitty{"Кот"}

	h, err := json.Marshal(cat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = io.WriteString(w, string(h))
	if err != nil {
		log.WithField("err", err).Error("Failed to write response")
	}
}

func helloWorld(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprintf(w, "Hello World")
	if err != nil {
		return
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(log.Fields{
			"method":     r.Method,
			"url":        r.URL.String(),
			"remoteAddr": r.RemoteAddr,
			"userAgent":  r.UserAgent(),
		}).Info("get a new request")
		next.ServeHTTP(w, r)
	})
}
