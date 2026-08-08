package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestOpt(addr, expectContent string) Opt {
	opt := Opt{
		Hostname:   addr,
		URI:        "/",
		Method:     "GET",
		Port:       80,
		Expect:     "HTTP/1.1 200",
		bufferSize: 1024,
	}
	if expectContent != "" {
		opt.expectByte = []byte(expectContent)
	}
	return opt
}

type runRequestOptFunc func(*Opt)

func runRequestTest(t *testing.T, handler http.HandlerFunc, modifyOpt runRequestOptFunc, wantCode int, wantSubstring string) {
	t.Helper()

	server := httptest.NewServer(handler)
	defer server.Close()

	opt := newTestOpt(server.Listener.Addr().String(), "")
	opt.Consecutive = 1
	opt.Interim = 10 * time.Millisecond
	if modifyOpt != nil {
		modifyOpt(&opt)
	}

	client := server.Client()
	msg, code := opt.runRequest(context.Background(), client)
	if code != wantCode {
		t.Fatalf("runRequest() code = %d, want %d; msg = %s", code, wantCode, msg)
	}
	if !strings.Contains(msg, wantSubstring) {
		t.Fatalf("runRequest() msg = %q, want substring %q", msg, wantSubstring)
	}
}

func TestRunRequestSuccess(t *testing.T) {
	runRequestTest(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "ok")
		}),
		nil,
		OK,
		"HTTP OK:",
	)
}

func TestRunRequestStatusNotMatched(t *testing.T) {
	runRequestTest(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, "error")
		}),
		nil,
		CRITICAL,
		"Invalid HTTP response received",
	)
}

func TestRunRequestContentNotMatched(t *testing.T) {
	runRequestTest(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "unexpected content")
		}),
		func(opt *Opt) {
			opt.expectByte = []byte("expected body")
		},
		CRITICAL,
		"Not matched",
	)
}

func TestRunRequestConsecutiveSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	opt := newTestOpt(server.Listener.Addr().String(), "")
	opt.Consecutive = 3
	opt.Interim = 10 * time.Millisecond

	client := server.Client()
	msg, code := opt.runRequest(context.Background(), client)
	if code != OK {
		t.Fatalf("runRequest() code = %d, want %d; msg = %s", code, OK, msg)
	}
	if callCount != 3 {
		t.Fatalf("server called %d times, want 3", callCount)
	}
	if !strings.HasPrefix(msg, "HTTP OK:") {
		t.Fatalf("runRequest() msg = %q, want HTTP OK prefix", msg)
	}
}

func TestRequestClientTimeout(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	defer close(block)

	opt := newTestOpt(server.Listener.Addr().String(), "")

	client := server.Client()
	client.Timeout = 50 * time.Millisecond

	_, errReq := opt.Request(context.Background(), client)
	if errReq == nil {
		t.Fatal("Request() error = nil, want non-nil")
	}
	if errReq.Code() != CRITICAL {
		t.Fatalf("Request() code = %d, want %d", errReq.Code(), CRITICAL)
	}
	if !strings.Contains(errReq.Error(), "context deadline exceeded") {
		t.Fatalf("Request() error = %q, want context deadline exceeded", errReq.Error())
	}
}

func TestRunWaitForEventuallySucceeds(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	opt := newTestOpt(server.Listener.Addr().String(), "")
	opt.Consecutive = 1
	opt.WaitFor = true
	opt.WaitForInterval = 10 * time.Millisecond
	opt.Interim = 10 * time.Millisecond

	client := server.Client()
	msg, code := opt.runWaitFor(context.Background(), client)
	if code != OK {
		t.Fatalf("runWaitFor() code = %d, want %d; msg = %s", code, OK, msg)
	}
	if callCount < 3 {
		t.Fatalf("server called %d times, want at least 3", callCount)
	}
	if !strings.HasPrefix(msg, "HTTP OK:") {
		t.Fatalf("runWaitFor() msg = %q, want HTTP OK prefix", msg)
	}
}

func TestRunWaitForContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	opt := newTestOpt(server.Listener.Addr().String(), "")
	opt.Consecutive = 1
	opt.WaitFor = true
	opt.WaitForInterval = 10 * time.Millisecond
	opt.Interim = 10 * time.Millisecond

	client := server.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	msg, code := opt.runWaitFor(ctx, client)
	if code != UNKNOWN {
		t.Fatalf("runWaitFor() code = %d, want %d; msg = %s", code, UNKNOWN, msg)
	}
	if msg != "Give up waiting for success" {
		t.Fatalf("runWaitFor() msg = %q, want %q", msg, "Give up waiting for success")
	}
}

func TestRequestWithContentMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "hello world")
	}))
	defer server.Close()

	opt := newTestOpt(server.Listener.Addr().String(), "world")

	client := server.Client()
	msg, errReq := opt.Request(context.Background(), client)
	if errReq != nil {
		t.Fatalf("Request() error = %v", errReq)
	}
	if !strings.Contains(msg, "Response body matched") {
		t.Fatalf("Request() msg = %q, want Response body matched", msg)
	}
}
