package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/backup"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/httpapi"
	"github.com/Buffer379x/puffolio/internal/media"
	"github.com/Buffer379x/puffolio/internal/store"
	"github.com/Buffer379x/puffolio/web"
	"golang.org/x/term"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"
)

var version = "dev"

func main() {
	if e := run(os.Args[1:]); e != nil {
		slog.Error("puffolio stopped", "error", e.Error())
		os.Exit(1)
	}
}
func run(args []string) error {
	configureMemoryLimit()
	c, e := config.Load()
	if e != nil {
		return e
	}
	command := "serve"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	switch command {
	case "--version", "version":
		fmt.Println(version)
		return nil
	case "healthcheck":
		return healthcheck(c)
	case "restore":
		f := flag.NewFlagSet("restore", flag.ContinueOnError)
		from := f.String("from", "", "instance backup ZIP")
		dest := f.String("data-dir", c.DataDir, "empty destination directory")
		if e = f.Parse(args); e != nil {
			return e
		}
		if *from == "" {
			return fmt.Errorf("--from is required")
		}
		return backup.Restore(context.Background(), *from, *dest)
	case "reset-password":
		return reset(c, args)
	case "import":
		return importCollection(c, args)
	case "serve":
		return serve(c, args)
	default:
		return fmt.Errorf("unknown command %q (serve, healthcheck, restore, reset-password, import, version)", command)
	}
}
func serve(c config.Config, args []string) error {
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	f.StringVar(&c.DataDir, "data-dir", c.DataDir, "data directory")
	f.StringVar(&c.Addr, "addr", c.Addr, "HTTP address")
	if e := f.Parse(args); e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s, e := store.Open(ctx, c)
	if e != nil {
		return e
	}
	defer s.Close()
	if _, e = os.Stat(c.DataDir + "/setup-code"); e == nil {
		slog.Info("first-run setup code is available in file", "path", c.DataDir+"/setup-code")
	}
	assets, e := fs.Sub(web.Dist, "dist")
	if e != nil {
		return e
	}
	handler := httpapi.New(httpapi.Dependencies{Store: s, Config: c, Web: assets, Version: version})
	srv := &http.Server{Addr: c.Addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: c.ReadTimeout, WriteTimeout: c.WriteTimeout, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	listener, e := net.Listen("tcp", c.Addr)
	if e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() {
		e := srv.Serve(listener)
		if errors.Is(e, http.ErrServerClosed) {
			e = nil
		}
		done <- e
	}()
	gcDone := make(chan struct{})
	go func() {
		defer close(gcDone)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if e := media.New(s).Cleanup(ctx, now); e != nil && ctx.Err() == nil {
					slog.Error("maintenance failed", "error", e.Error())
				}
			}
		}
	}()
	slog.Info("puffolio listening", "address", listener.Addr().String(), "version", version)
	select {
	case e = <-done:
		stop()
		<-gcDone
		return e
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		e = srv.Shutdown(shutdownCtx)
		if e != nil {
			srv.Close()
		}
		<-gcDone
		if serveErr := <-done; e == nil {
			e = serveErr
		}
		return e
	}
}
func healthcheck(c config.Config) error {
	host, port, e := net.SplitHostPort(c.Addr)
	if e != nil {
		return fmt.Errorf("invalid HTTP address: %w", e)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/readyz", nil)
	if e != nil {
		return e
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, e := client.Do(req)
	if e != nil {
		return e
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode != 200 {
		return fmt.Errorf("readiness status %d", response.StatusCode)
	}
	return nil
}
func reset(c config.Config, args []string) error {
	f := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	username := f.String("username", "", "account username")
	f.StringVar(&c.DataDir, "data-dir", c.DataDir, "data directory")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *username == "" {
		return fmt.Errorf("--username is required")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("reset-password requires an interactive terminal")
	}
	fmt.Fprint(os.Stderr, "Neues Passwort: ")
	p, e := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if e != nil {
		return e
	}
	fmt.Fprint(os.Stderr, "Passwort wiederholen: ")
	confirmation, e := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if e != nil {
		return e
	}
	if !auth.Equal(string(p), string(confirmation)) {
		return fmt.Errorf("passwords do not match")
	}
	s, e := store.Open(context.Background(), c)
	if e != nil {
		return e
	}
	defer s.Close()
	return auth.New(s).ResetPassword(context.Background(), *username, string(p))
}
func importCollection(c config.Config, args []string) error {
	f := flag.NewFlagSet("import", flag.ContinueOnError)
	from := f.String("from", "", "collection ZIP")
	cid := f.String("collection", "", "destination collection UUID")
	f.StringVar(&c.DataDir, "data-dir", c.DataDir, "data directory")
	expanded := f.Int64("max-expanded-bytes", 0, "explicit expanded byte limit")
	entries := f.Int("max-entries", 0, "explicit archive entry limit")
	images := f.Int("max-images", 0, "explicit image count limit")
	if e := f.Parse(args); e != nil {
		return e
	}
	if *from == "" || *cid == "" || *expanded <= 0 || *entries <= 0 || *images <= 0 {
		return fmt.Errorf("--from, --collection and all three positive import limits are required")
	}
	s, e := store.Open(context.Background(), c)
	if e != nil {
		return e
	}
	defer s.Close()
	counts, e := backup.Import(context.Background(), s, *cid, *from, backup.Limits{MaxExpandedBytes: *expanded, MaxEntries: *entries, MaxImages: *images})
	if e != nil {
		return e
	}
	fmt.Println("Imported " + strconv.Itoa(counts.Liquids) + " liquids, " + strconv.Itoa(counts.Fields) + " fields, " + strconv.Itoa(counts.Media) + " images")
	return nil
}
