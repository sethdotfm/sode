// Command sode serves a keyboard-first SW-P-08 patching page.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sethdotfm/sode/internal/app"
	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/web"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("%s: %v", k, err)
	}
	return n
}

func envBool(k string) bool {
	b, _ := strconv.ParseBool(os.Getenv(k))
	return b
}

// levels builds the UI → wire level map. UI level 1 is video, 2.. are
// audio; wire = UI + SODE_LEVEL_WIRE_OFFSET. SODE_LEVEL_LABELS overrides
// the labels as a comma list.
func levels() []app.Level {
	count := envInt("SODE_LEVEL_COUNT", 17)
	off := envInt("SODE_LEVEL_WIRE_OFFSET", -1)
	var labels []string
	if s := os.Getenv("SODE_LEVEL_LABELS"); s != "" {
		labels = strings.Split(s, ",")
	}
	out := make([]app.Level, count)
	for i := range out {
		ui := i + 1
		l := app.Level{UI: ui, Wire: ui + off, Label: "Video", Swatch: "video"}
		if ui > 1 {
			l.Label = fmt.Sprintf("A%d", ui-1)
			l.Swatch = fmt.Sprintf("%02d", (ui-2)%16+1)
		}
		if i < len(labels) && strings.TrimSpace(labels[i]) != "" {
			l.Label = strings.TrimSpace(labels[i])
		}
		if l.Wire < 0 || l.Wire > 255 {
			log.Fatalf("level %d maps to wire level %d; check SODE_LEVEL_WIRE_OFFSET", ui, l.Wire)
		}
		out[i] = l
	}
	return out
}

func main() {
	health := flag.Bool("healthcheck", false, "probe the local /healthz and exit")
	flag.Parse()
	listen := env("SODE_LISTEN", ":8080")

	if *health {
		_, port, _ := net.SplitHostPort(listen)
		cl := http.Client{Timeout: 2 * time.Second}
		resp, err := cl.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	router := os.Getenv("SODE_ROUTER")
	if router == "" {
		log.Fatal("SODE_ROUTER is required (host:port of the SW-P-08 router)")
	}
	cfg := app.Config{
		Router:        router,
		Matrix:        envInt("SODE_MATRIX", 0),
		Levels:        levels(),
		DefaultLevels: env("SODE_DEFAULT_LEVELS", ""),
		NameLen:       envInt("SODE_NAME_LEN", 12),
		ExtNames:      env("SODE_NAMES", "std") == "ext",
		NumOffset:     envInt("SODE_NUM_OFFSET", 1),
		LockoutMs:     envInt("SODE_CONFIRM_LOCKOUT_MS", 500),
		ConfirmEnter:  envBool("SODE_CONFIRM_ENTER"),
	}
	if cfg.DefaultLevels == "" {
		cfg.DefaultLevels = fmt.Sprintf("1-%d", len(cfg.Levels))
	}

	sw := &swp08.Client{Addr: router, Debug: envBool("SODE_DEBUG")}
	a, err := app.New(cfg, sw)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go sw.Run(ctx)
	go a.Run(ctx)

	srv := &http.Server{Addr: listen, Handler: a.Handler(web.FS), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	log.Printf("sode: http on %s, router %s matrix %d, %d levels (UI 1 → wire %d)",
		listen, router, cfg.Matrix, len(cfg.Levels), cfg.Levels[0].Wire)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
