// Command sode serves a keyboard-first SW-P-08 patching page.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sethdotfm/sode/internal/app"
	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadConfig reads the YAML file. On first start (no file) it is created
// from the SODE_* seed variables; after that the file wins.
func loadConfig(path string) config.Config {
	if path != "" {
		c, exists, err := config.Load(path)
		if err != nil {
			log.Fatalf("config: %v", err)
		}
		if exists && c.SettingsPin != "" && !config.ValidPinHash(c.SettingsPin) {
			log.Fatalf("config: settingsPin in %s isn't a PIN hash (set PINs from Settings, not by hand); run sode -reset-pin", path)
		}
		if exists {
			log.Printf("config: loaded %s (SODE_* seed variables are ignored once it exists)", path)
			return c
		}
	}
	c, err := config.FromEnv(config.Defaults())
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		log.Fatalf("config from environment: %v", err)
	}
	if path == "" {
		log.Printf("config: SODE_CONFIG not set; settings changes will not survive a restart")
		return c
	}
	if err := config.Save(path, c); err != nil {
		log.Fatalf("config: writing %s: %v", path, err)
	}
	log.Printf("config: created %s from environment", path)
	return c
}

// resetSettingsPin clears the PIN in the config file. A running server
// keeps its copy until it restarts.
func resetSettingsPin(path string) {
	if path == "" {
		log.Fatal("reset-pin: SODE_CONFIG is not set")
	}
	c, exists, err := config.Load(path)
	if err != nil {
		log.Fatalf("reset-pin: %v", err)
	}
	if !exists || c.SettingsPin == "" {
		log.Printf("reset-pin: no settings PIN in %s", path)
		return
	}
	c.SettingsPin = ""
	if err := config.Save(path, c); err != nil {
		log.Fatalf("reset-pin: %v", err)
	}
	log.Printf("reset-pin: removed the settings PIN from %s; restart sode to apply", path)
}

func main() {
	health := flag.Bool("healthcheck", false, "probe the local /healthz and exit")
	resetPin := flag.Bool("reset-pin", false, "remove the settings PIN from SODE_CONFIG and exit (restart sode afterwards)")
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

	cfgPath := os.Getenv("SODE_CONFIG")
	if *resetPin {
		resetSettingsPin(cfgPath)
		return
	}
	cfg := loadConfig(cfgPath)
	if cfg.SettingsPin != "" {
		log.Printf("config: settings PIN is set (remove it with sode -reset-pin)")
	}

	hist, err := app.OpenHistory(os.Getenv("SODE_HISTORY"))
	if err != nil {
		log.Fatalf("history: %v", err)
	}
	sentMB, err := strconv.Atoi(env("SODE_SENTLOG_MB", "64"))
	if err != nil || sentMB < 1 {
		log.Fatalf("SODE_SENTLOG_MB: want a whole number of megabytes, at least 1")
	}
	sent, err := app.OpenSentLog(os.Getenv("SODE_SENTLOG"), int64(sentMB)<<20)
	if err != nil {
		log.Fatalf("sent log: %v", err)
	}
	up, err := app.OpenUptime(os.Getenv("SODE_UPTIME"))
	if err != nil {
		log.Fatalf("uptime: %v", err)
	}
	debug, _ := strconv.ParseBool(os.Getenv("SODE_DEBUG"))
	a, err := app.New(cfg, cfgPath, hist, sent, debug)
	if err != nil {
		log.Fatal(err)
	}
	a.SetUptime(up)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ran := make(chan struct{})
	go func() { a.Run(ctx); close(ran) }()

	srv := &http.Server{Addr: listen, Handler: a.Handler(web.FS), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	log.Printf("sode: http on %s, router %q matrix %d, %d levels (UI 1 → wire %d)",
		listen, cfg.Router, cfg.Matrix, cfg.Levels, 1+cfg.LevelWireOffset)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	// Run writes the stop row and the uptime's off line as it ends.
	select {
	case <-ran:
	case <-time.After(3 * time.Second):
	}
}
