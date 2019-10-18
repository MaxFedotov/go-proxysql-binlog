package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/siddontang/go-log/log"
)

var (
	clientsConnected = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "proxysql_binlog_clients_connected_total",
		Help: "Total number of clients connected to proxysql-binlog",
	})
	gtidProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "proxysql_binlog_gtid_processed_total",
		Help: "Total number of GTIDs processed by proxysql-binlog",
	})
	clientErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "proxysql_binlog_client_erros_total",
		Help: "Total number of client connection errors",
	})
	readerErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "proxysql_binlog_reader_erros_total",
		Help: "Total number of binlog reader errors",
	})
)

func NewMetricsServer() (metrics *http.Server) {
	http.Handle(Config.Metrics.Endpoint, promhttp.Handler())
	metrics = &http.Server{Addr: Config.Metrics.ListenAddress, Handler: nil}
	go func() {
		wg.Add(1)
		log.Infof("Starting HTTP metrics server on %v%s", Config.Metrics.ListenAddress, Config.Metrics.Endpoint)
		err := metrics.ListenAndServe()
		if err != nil {
			if err == http.ErrServerClosed {
				log.Info("Stopping HTTP metrics server")
				wg.Done()
				return
			}
			log.Fatalf("unable to start metrics HTTP server: %v", err)
			wg.Done()
		}
	}()
	return metrics
}
