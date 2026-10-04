package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Only the optimised assets are embedded; the full-size originals in
// static/images stay out of the binary. content.json is the site's text.
//
//go:embed content.json templates static/css static/js static/fonts static/images/opt
var embedded embed.FS

func main() {
	addr := flag.String("addr", envOr("ADDR", "127.0.0.1:8080"), "listen address")
	dev := flag.Bool("dev", os.Getenv("DEV") == "1", "serve templates, content.json and static files from disk and re-read templates and content on every request")
	trustProxy := flag.Bool("trust-proxy", os.Getenv("TRUST_PROXY") == "1", "take the client IP from X-Forwarded-For (only behind a reverse proxy)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var assets fs.FS = embedded
	if *dev {
		assets = os.DirFS(".")
	}

	app, err := newApp(assets, *dev, *trustProxy, logger)
	if err != nil {
		logger.Error("init", "err", err)
		os.Exit(1)
	}

	srv := newServer(*addr, app.routes())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("listening", "addr", *addr, "dev", *dev)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
}

// newServer bounds every phase of a connection so slow or oversized clients
// can't hold resources open (slowloris and friends).
func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
