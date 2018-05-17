package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

type HttpServer struct {
	address    string
	log        *log.Logger
	httpServer *http.Server
	monitor    *GoMonErrs
}

func NewHttpServer(address string, logger *log.Logger) *HttpServer {

	h := &HttpServer{
		address: address,
		log:     logger,
	}
	return h
}

func (h *HttpServer) newRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/status", h.statusHandler).Methods("GET")
	r.HandleFunc("/hosts", h.hostsHandler).Methods("GET")
	r.HandleFunc("/checks", h.checksHandler).Methods("GET")
	r.HandleFunc("/handlers", h.handlersHandler).Methods("GET")

	staticFileDirectory := http.Dir("./assets/")
	staticFileHandler := http.StripPrefix("/assets/", http.FileServer(staticFileDirectory))
	// Get all routes starting with /assets/
	r.PathPrefix("/assets/").Handler(staticFileHandler).Methods("GET")
	return r
}

func (h *HttpServer) startHttp(g *GoMonErrs) {
	h.monitor = g
	r := h.newRouter()
	h.httpServer = &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go h.httpServer.ListenAndServe()
}

func (h *HttpServer) stopHttp() {
	h.httpServer.Close()
}

func (h *HttpServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	hs := h.monitor.getStatus()
	h.marshallJsonAndReply(hs, w, r)
}

func (h *HttpServer) hostsHandler(w http.ResponseWriter, r *http.Request) {
	hs := h.monitor.hostsCfg
	if hs.Hosts == nil {
		fmt.Fprintf(w, "{\"Hosts\": [] }")
		return
	}
	h.marshallJsonAndReply(hs, w, r)
}

func (h *HttpServer) checksHandler(w http.ResponseWriter, r *http.Request) {
	hs := h.monitor.checksCfg
	if hs.Checks == nil {
		fmt.Fprintf(w, "{\"Checks\": [] }")
		return
	}
	h.marshallJsonAndReply(hs, w, r)
}

func (h *HttpServer) handlersHandler(w http.ResponseWriter, r *http.Request) {
	hs := h.monitor.handlersCfg
	if hs.Handlers == nil {
		fmt.Fprintf(w, "{\"Handlers\": [] }")
		return
	}
	h.marshallJsonAndReply(hs, w, r)
}

func (h *HttpServer) marshallJsonAndReply(v interface{}, w http.ResponseWriter, r *http.Request) {
	hsBytes, err := json.MarshalIndent(v, "", "  ")
	//hsBytes, err := json.Marshal(hs)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "{ \"error\": \"Failed to marshal JSON.\"}")
		return
	}
	w.Write(hsBytes)
}
