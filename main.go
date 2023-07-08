package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/willglynn/greatriverenergy_exporter/greatriverenergy/exporter"
)

func main() {
	rt := http.DefaultTransport

	realtime := prometheus.NewRegistry()
	realtime.MustRegister(exporter.NewRealtime(rt))

	opts := promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(realtime, opts))

	mux.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		days, _ := strconv.Atoi(query.Get("days"))
		if days < 1 {
			days = 7
		}

		reg := prometheus.NewRegistry()
		reg.MustRegister(exporter.NewHistory(rt, days))
		promhttp.HandlerFor(reg, opts).ServeHTTP(w, r)
	})

	var addr string
	addr = os.Getenv("LISTEN")
	if port := os.Getenv("PORT"); addr == "" && port != "" {
		addr = ":" + port
	}
	if addr == "" {
		addr = ":2024"
	}

	log.Printf("Starting HTTP server on %v", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Error starting HTTP server: %v", err)
	}
}
