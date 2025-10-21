package lib

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func InitMetrics(ipBlocker ClientFilter) (*http.Server, *Metrics) {
	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled, labeled by domain and status.",
		},
		[]string{"domain", "status"},
	)

	blockedTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blocked_clients_total",
			Help: "Blocked ips and type of block",
		},
		[]string{"type", "ip"},
	)

	prometheus.MustRegister(requestsTotal, blockedTotal)
	metr := &Metrics{
		RequestsTotal: requestsTotal,
		BlockedTotal:  blockedTotal,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/clearip", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		ipBlocker.Reset()
	})

	return &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       180 * time.Second,
	}, metr
}

func InitServer(ipb ClientFilter, vars *EnvVars, me *Metrics, rt map[string]url.URL) *http.Server {
	bucket := NewTokenBucket(vars.BucketLimit, vars.BucketRate)

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			target := rt[CutPort(r.In.Host)]
			r.SetURL(&target)
			r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
			r.SetXForwarded()
		},
	}

	handler := NewRouter(proxy, bucket, ipb, me, rt)

	return &http.Server{
		Addr:              ":80",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
