package lib

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	lIP    = "ip"
	lRate  = "ratelimit"
	lRoute = "notrouted"
)

type proxyResponseWriter struct {
	w http.ResponseWriter
	f ClientFilter
	i string
	h string
	m *Metrics
}

func (p *proxyResponseWriter) Header() http.Header {
	return p.w.Header()
}

func (p *proxyResponseWriter) Write(data []byte) (int, error) {
	return p.w.Write(data)
}
func (p *proxyResponseWriter) WriteHeader(statusCode int) {
	if statusCode > 399 {
		slog.Warn("User got blocked", "code", statusCode, lIP, p.i)
		p.m.Blocked(lIP, p.i, p.h, strconv.Itoa(statusCode))
		p.f.NotifyFailure(p.i)
	} else {
		p.m.RequestsTotal.WithLabelValues(p.h, strconv.Itoa(statusCode)).Inc()
	}
	p.w.WriteHeader(statusCode)
}

type Router struct {
	handler http.Handler
	bucket  Bucket
	clientF ClientFilter
	metrics *Metrics
	routing map[string]url.URL
}

func NewRouter(h http.Handler, b Bucket, c ClientFilter, m *Metrics, r map[string]url.URL) *Router {
	return &Router{
		handler: h,
		bucket:  b,
		clientF: c,
		metrics: m,
		routing: r,
	}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := rt.getClientIP(r)
	host := CutPort(r.Host)

	if rt.clientF.CheckBlocked(ip) {
		rt.clientF.NotifyFailure(ip)
		slog.Error("blocked ip", "val", ip)
		w.WriteHeader(414)
		rt.metrics.Blocked(lIP, ip, host, "414")
		return
	}
	if !rt.bucket.GetToken() {
		slog.Error("rate limited")
		w.WriteHeader(414)
		rt.metrics.Blocked(lRate, ip, host, "415")
		return
	}
	_, ok := rt.routing[host]
	if !ok {
		slog.Error("routing not found", "val", host)
		w.WriteHeader(http.StatusNotFound)
		rt.metrics.Blocked(lRoute, ip, host, strconv.Itoa(http.StatusNotFound))
		return
	}

	rt.handler.ServeHTTP(&proxyResponseWriter{w: w, f: rt.clientF, i: ip, h: host, m: rt.metrics}, r)
}

func (rt *Router) getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	if xRealIP := r.Header.Get("X-Real-Ip"); xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	idx := strings.LastIndex(r.RemoteAddr, ":")
	if idx >= 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

func CutPort(url string) string {
	if idx := strings.LastIndex(url, ":"); idx != -1 {
		return url[:idx]
	}
	return url
}
