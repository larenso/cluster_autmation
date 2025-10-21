package lib

import (
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type EnvVars struct {
	PbKey       string        `env:"PB_API"`
	PbSecret    string        `env:"PB_SECRET"`
	Target      url.URL       `env:"FINAL_TARGET" envDefault:"http://immich"`
	ConfMap     string        `env:"CFMAP_IP" envDefault:"public-ip"`
	BucketLimit int           `env:"BUCKET_LIMIT" envDefault:"10"`
	BucketRate  int           `env:"BUCKET_RATE" envDefault:"2"`
	IPLimit     int           `env:"IP_LIMIT" envDefault:"4"`
	IPDuration  time.Duration `env:"IP_DURATION" envDefault:"2h"`
	DNSRecheck  time.Duration `env:"DNS_RECHECK" envDefault:"10m"`
	Namespace   string        `env:"NAMESPACE"`
}

type Metrics struct {
	RequestsTotal *prometheus.CounterVec
	BlockedTotal  *prometheus.CounterVec
}

func (m *Metrics) Blocked(reason, ip, host, status string) {
	m.BlockedTotal.WithLabelValues(reason, ip).Inc()
	m.RequestsTotal.WithLabelValues(host, status).Inc()
}
