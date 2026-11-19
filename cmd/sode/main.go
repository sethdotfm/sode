// Command sode serves a keyboard-first SW-P-08 patching page.
package main

import (
	"bufio"
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
	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func loadConfig(path string) config.Config {
	if path != "" {
		backupBeforeRelease(path)
		c, exists, err := config.Load(path)
		if err != nil {
			log.Fatalf("config: %v", err)
		}
		if exists {
			if p := c.UnknownKeysProblem(); p != "" {
				log.Printf("config: %s", p)
			}
			log.Printf("config: loaded %s (SODE_* seed variables are ignored once it exists)", path)
			return c
		}
	}
	c, err := config.FromEnv(config.Defaults())
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	// The setup wizard and env-only mode apply it; a typo stops here, not at the wizard.
	if _, err := config.AccessPresetFromEnv(); err != nil {
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

// Runs before anything here can save, including the reset commands and Load. A missing file
// is nothing to back up.
func backupBeforeRelease(path string) {
	if err := config.BackupOnce(path, app.Version); err != nil {
		log.Printf("config: backing up %s: %v", path, err)
	}
	if err := config.BackupUnknown(path, time.Now()); err != nil {
		log.Printf("config: keeping a copy of %s with unknown keys: %v", path, err)
	}
}

// The PIN comes from stdin, not a flag, so it stays out of ps and shell history. With no
// terminal package (no new deps) the typed PIN shows.
func makeAdministrator(path, name string) {
	if path == "" {
		log.Fatal("make-administrator: SODE_CONFIG is not set")
	}
	fmt.Fprintf(os.Stderr, "New PIN for %s (digits; it shows as you type): ", name)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		log.Fatalf("make-administrator: reading the PIN: %v", err)
	}
	backupBeforeRelease(path)
	created, err := config.MakeAdministrator(path, name, strings.TrimSpace(line))
	if err != nil {
		log.Fatalf("make-administrator: %v", err)
	}
	what := "is now an Administrator with the new PIN"
	if created {
		what = "was added as an Administrator with the new PIN"
	}
	// A save from the running server before the restart would write its old users back.
	log.Printf("make-administrator: %s %s in %s. Restart sode now to apply it. Their API keys end at the restart.", name, what, path)
}

// A running server keeps its setting until it restarts.
func resetRequireHTTPS(path string) {
	if path == "" {
		log.Fatal("reset-https: SODE_CONFIG is not set")
	}
	backupBeforeRelease(path)
	was, err := config.ClearRequireHTTPS(path)
	if err != nil {
		log.Fatalf("reset-https: %v", err)
	}
	if !was {
		log.Printf("reset-https: Require HTTPS is not on in %s", path)
		return
	}
	log.Printf("reset-https: turned Require HTTPS off in %s; restart sode to apply", path)
}

func main() {
	health := flag.Bool("healthcheck", false, "probe the local /healthz and exit")
	makeAdmin := flag.String("make-administrator", "", "make the named user an Administrator in SODE_CONFIG and exit. It reads the new PIN from stdin. Restart sode afterwards")
	resetHTTPS := flag.Bool("reset-https", false, "turn Require HTTPS off in SODE_CONFIG and exit (restart sode afterwards)")
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
	if *makeAdmin != "" {
		makeAdministrator(cfgPath, *makeAdmin)
		return
	}
	if *resetHTTPS {
		resetRequireHTTPS(cfgPath)
		return
	}
	// Before anything opens a file the move takes.
	if _, err := app.MoveV1Files(cfgPath, os.Getenv("SODE_HISTORY"), os.Getenv("SODE_COMMLOG"), os.Getenv("SODE_UPTIME")); err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg := loadConfig(cfgPath)
	if len(cfg.Users) > 0 && !cfg.HasAdministrator() {
		log.Printf("config: no Administrator, so Administrator actions are refused; make one with sode -make-administrator NAME")
	}

	hist, err := app.OpenHistory(os.Getenv("SODE_HISTORY"))
	if err != nil {
		log.Fatalf("history: %v", err)
	}
	sentMB, err := strconv.Atoi(env("SODE_COMMLOG_MB", "64"))
	if err != nil || sentMB < 1 {
		log.Fatalf("SODE_COMMLOG_MB: want a whole number of megabytes, at least 1")
	}
	sent, err := app.OpenSentLog(os.Getenv("SODE_COMMLOG"), int64(sentMB)<<20)
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
		listen, cfg.Address, cfg.Matrix, cfg.Levels, 1+cfg.LevelWireOffset)
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	if err := srv.Serve(app.KeepAliveListener(ln)); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	// Run writes the stop row and the uptime's off line as it ends.
	select {
	case <-ran:
	case <-time.After(8 * time.Second): // Run waits up to 6s for takes and saves in flight
	}
}
