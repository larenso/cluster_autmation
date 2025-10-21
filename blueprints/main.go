package main

import (
	"context"
	"larenso/cluster_autmation/ratelimiter/lib"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	env "github.com/caarlos0/env/v11"
)

func getRoutes() map[string]url.URL {
	routes := make(map[string]url.URL)
	for _, envVar := range os.Environ() {
		if !strings.HasPrefix(envVar, "RO_") {
			continue
		}
		k, v, f := strings.Cut(envVar, "=")
		if !f || len(k) < 4 {
			continue
		}
		if !strings.HasPrefix(v, "http") {
			v = "http://" + v
		}
		url, err := url.Parse(v)
		if err != nil {
			slog.Error("parsing url, skippig", "val", err)
			continue
		}

		// cut RO_ , replace SUB_DOMAIN -> SUB.DOMAIN, to lowercase
		routes[strings.ToLower(strings.ReplaceAll(k[3:], "_", "."))] = *url
	}

	return routes
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var vars lib.EnvVars
	err := env.Parse(&vars)
	if err != nil {
		slog.Error("Error reading env vars", "val", err)
		return
	}

	routing := getRoutes()

	ipBlocker := lib.NewIPBlocker(vars.IPLimit, vars.IPDuration)
	metrics, metr := lib.InitMetrics(ipBlocker)
	server := lib.InitServer(ipBlocker, &vars, metr, routing)

	errch := make(chan error, 1)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		if err := server.ListenAndServe(); err != nil {
			errch <- err
		}
	}()

	go func() {
		if err := metrics.ListenAndServe(); err != nil {
			errch <- err
		}
	}()

	select {
	case err = <-errch:
		slog.Error(err.Error())
	case <-ctx.Done():
	}

	lctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to gracefully shut down the server
	if err = server.Shutdown(lctx); err != nil {
		slog.Error("server shutdown error", "val", err.Error())
	}

	if err = metrics.Shutdown(lctx); err != nil {
		slog.Error("metric server shutdown error", "val", err.Error())
	}
}
