package httpapi

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type stalledAuthBody struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *stalledAuthBody) Read([]byte) (int, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return 0, io.EOF
}
func (b *stalledAuthBody) Close() error { return nil }

func TestAnonymousAuthBodyDoesNotHoldWriter(t *testing.T) {
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/setup"} {
		t.Run(path, func(t *testing.T) {
			c, _ := setupClient(t)
			// Login/invite must finish parsing before entering the shared writer boundary.
			b := &stalledAuthBody{entered: make(chan struct{}), release: make(chan struct{})}
			r := httptest.NewRequest("POST", "http://localhost"+path, b)
			r.Header.Set("Origin", "http://localhost")
			r.Header.Set("Content-Type", "application/json")
			done := make(chan struct{})
			go func() { c.h.ServeHTTP(httptest.NewRecorder(), r); close(done) }()
			defer func() { close(b.release); <-done }()
			if path != "/api/v1/setup" {
				select {
				case <-b.entered:
				case <-time.After(time.Second):
					t.Fatal("body was not read")
				}
			}
			mutation := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				req := httptest.NewRequest("POST", "http://localhost/api/v1/liquids", strings.NewReader(`{"name":"Independent mutation"}`))
				req.Header.Set("Origin", "http://localhost")
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-CSRF-Token", c.csrf)
				req.AddCookie(c.cookie)
				w := httptest.NewRecorder()
				c.h.ServeHTTP(w, req)
				mutation <- w
			}()
			select {
			case w := <-mutation:
				if w.Code != 201 {
					t.Fatal(w.Code, w.Body.String())
				}
			case <-time.After(time.Second):
				t.Error("unfinished anonymous body blocked independent authorized mutation")
			}
		})
	}
}

func TestCanceledWriterWaiterReturnsWithoutMutation(t *testing.T) {
	c, s := setupClient(t)
	s.Writer.Lock()
	defer s.Writer.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", strings.NewReader(`{"username":"alice","password":"A-long-test-password"}`)).WithContext(ctx)
	req.Header.Set("Origin", "http://localhost")
	req.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	go func() { c.h.ServeHTTP(httptest.NewRecorder(), req); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled waiter retained until another writer releases lock")
	}
	var n int
	if e := s.DB.QueryRow("SELECT count(*) FROM sessions").Scan(&n); e != nil || n != 1 {
		t.Fatal("canceled login created session", n, e)
	}
}

func TestPublicAuthRejectsInvalidBodiesBeforeWriter(t *testing.T) {
	c, s := setupClient(t)
	s.Writer.Lock()
	defer s.Writer.Unlock()
	for _, path := range []string{"/api/v1/auth/login"} {
		for _, test := range []struct {
			body, contentType string
			status            int
		}{
			{`{}`, "text/plain", 415},
			{`{"unknown":true}`, "application/json", 400},
			{`{"password":"` + strings.Repeat("x", 9<<10) + `"}`, "application/json", 413},
			{`{} {}`, "application/json", 400},
		} {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			req := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader(test.body)).WithContext(ctx)
			req.Header.Set("Origin", "http://localhost")
			req.Header.Set("Content-Type", test.contentType)
			w := httptest.NewRecorder()
			c.h.ServeHTTP(w, req)
			cancel()
			if w.Code != test.status {
				t.Fatalf("%s invalid body waited for writer or was accepted: got %d want %d", path, w.Code, test.status)
			}
		}
	}
}
