package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/datainq/sdfmt"
	"github.com/go-chi/chi"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func getAndLog(m map[string]string, name, arg string) (string, bool) {
	value, ok := os.LookupEnv(name)
	if !ok {
		logrus.Warnf("ENV: %q missing", name)
	} else if value == "" {
		logrus.Warnf("ENV: %q present, but value is empty", name)
	} else {
		logrus.Infof("ENV: %q present and value is NOT empty", name)
		m[arg] = value
	}
	return value, ok
}

// Postgres has multiple params, you can find documentation here:
//  - https://godoc.org/github.com/lib/pq
//  - https://jdbc.postgresql.org/documentation/head/connect.html
func maybeConnectDb(log logrus.FieldLogger, wg *sync.WaitGroup, done chan bool) {
	connParams := map[string]string{
		"connect_timeout": "10", // Without it, the binary will hang long time waiting for a database connection.
	}
	dbHost, ok := getAndLog(connParams, "DB_ADDR", "host")
	var callback func()
	if ok && dbHost != "" {
		log.Info("will try connect to database")
		getAndLog(connParams, "DB_PORT", "port")
		getAndLog(connParams, "DB_DATABASE", "dbname")
		getAndLog(connParams, "DB_USER", "user")
		getAndLog(connParams, "DB_PASSWORD", "password")
		sslMode, _ := getAndLog(connParams, "DB_SSL_MODE", "sslmode")
		if sslMode == "allow" || sslMode == "prefer" || sslMode == "require" ||
			sslMode == "verify-ca" || sslMode == "verify-full" {

			getAndLog(connParams, "DB_SSL_CERT", "sslcert")
			getAndLog(connParams, "DB_SSL_KEY", "sslkey")
			getAndLog(connParams, "DB_SSL_ROOT_CERT", "sslrootcert")
		} else {
			connParams["sslmode"] = "disable"
		}

		builder := &strings.Builder{}
		params := &strings.Builder{}
		for k, v := range connParams {
			builder.WriteString(k)
			builder.WriteRune('=')
			builder.WriteString(v)
			builder.WriteRune(' ')

			params.WriteString(k)
			params.WriteRune(',')
		}
		connStr := builder.String()
		log.Infof("database connection string has params: %s", params)
		callback = func() {
			pingDatabase(log, connStr)
		}
	} else {
		log.Info("no database host, won't try connect")
		callback = func() {
			promDbPingTotal.WithLabelValues("skip").Inc()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-time.After(10 * time.Second):
				callback()
			case <-done:
				log.Info("db connection and ping loop ended")
				return
			}
		}
	}()
}

func pingDatabase(log logrus.FieldLogger, connStr string) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.WithError(err).Error("connect to database")
		return
	}
	log.Info("connected to database")

	ctx, canc := context.WithTimeout(context.Background(), 10*time.Second)
	defer canc()
	if err := db.PingContext(ctx); err != nil {
		promDbPingTotal.WithLabelValues("fail").Inc()
		log.WithError(err).Error("ping database")
		return
	}

	promDbPingTotal.WithLabelValues("success").Inc()
	if _, err := db.Query("SELECT 1"); err != nil {
		log.WithError(err).Error("database cannot SELECT 1")
	}

	if err = db.Close(); err != nil {
		log.WithError(err).Error("closing database connection")
	}
}

var (
	promDbPingTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "database_ping_total",
			Help: "Total number of database ping sent.",
		},
		[]string{"status"},
	)
	promHttpReqTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP request by code.",
		},
		[]string{"code", "method"},
	)
)

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		TLSConfig:         nil,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
}

func startHTTP(log logrus.FieldLogger, addr string, wg *sync.WaitGroup, done chan bool) (*chi.Mux, *http.Server) {
	m := chi.NewMux()
	m.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		log.WithField("listen", addr).Infof("request %s", r.URL.String())
		data, err := httputil.DumpRequest(r, false)
		if err != nil {
			log.WithError(err).Error("dumping request")
		} else {
			w.Header().Set("Content-Type", "text/plain")
			if n, err := w.Write(data); err != nil {
				log.WithError(err).Errorf("writing content (wrote %d bytes)", n)
			}
		}
	})
	if metricsPath, ok := os.LookupEnv("METRICS_PATH"); ok {
		if metricsPath != "" {
			log.Infof("register metrics at: %s%s", addr, metricsPath)
			m.Get(metricsPath, promhttp.Handler().ServeHTTP)
		} else {
			log.Info("skip metrics registration")
		}
	} else {
		log.Info("register metrics at: /metrics")
		m.Get("/metrics", promhttp.Handler().ServeHTTP)
	}

	s := newServer(addr, promhttp.InstrumentHandlerCounter(promHttpReqTotal, m))
	go func() {
		<-done
		log.Infof("got done, closing %s", s.Addr)
		if err := s.Shutdown(context.Background()); err != nil {
			log.WithError(err).Error("nice shutdown of: %s", s.Addr)
		}
	}()
	wg.Add(1)
	log.Infof("Listen at %s", s.Addr)
	go func() {
		defer wg.Done()
		if err := s.ListenAndServe(); err == http.ErrServerClosed {
			log.Infof("graceful shutdown: %s", s.Addr)
		} else if err != nil {
			log.WithError(err).Errorf("server: %s", s.Addr)
		}
	}()
	return m, s
}

func maybeStartHTTP(log logrus.FieldLogger, addrs []string, wg *sync.WaitGroup, done chan bool) {
	for _, addr := range addrs {
		startHTTP(log, addr, wg, done)
	}
}

func startPreStop(log logrus.FieldLogger, addr string, wg *sync.WaitGroup, done chan bool) {
	m, _ := startHTTP(log, addr, wg, done)
	m.Get("/k8s/preStop", func(w http.ResponseWriter, r *http.Request) {
		log.Info("Kubernetes requested us to stop")
		close(done)
		wg.Wait()
		w.WriteHeader(http.StatusNoContent)
	})
}

func main() {
	log := logrus.StandardLogger()
	if _, onKubernetes := os.LookupEnv("KUBERNETES_SERVICE_HOST"); onKubernetes {
		log.SetFormatter(&sdfmt.StackdriverFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			ForceColors: true,
		})
	}

	wg := &sync.WaitGroup{}
	done := make(chan bool, 1)
	maybeConnectDb(log, wg, done)

	addrs := []string{":80", ":8080"}
	if listen, ok := os.LookupEnv("LISTEN_ADDR"); ok {
		addrs = nil
		for _, v := range strings.Split(listen, ",") {
			addrs = append(addrs, strings.TrimSpace(v))
		}
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println(sig)
		close(done)
	}()

	maybeStartHTTP(log, addrs, wg, done)
	if addr, ok := os.LookupEnv("PRE_STOP_ADDR"); ok {
		log.Infof("will start preStop hook at %s/k8s/preStop", addr)
		startPreStop(log, addr, wg, done)
	} else {
		log.Info("no PRE_STOP_ADDR will NOT start preStop hook")
	}

	wg.Wait()
	log.Info("stop execution")
}
