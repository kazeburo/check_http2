package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
)

// Version by Makefile
var version string

type commandOpts struct {
	Timeout       time.Duration `long:"timeout" default:"300s" description:"Timeout to wait for connection"`
	Hostname      string        `short:"H" long:"hostname" description:"Host name using Host headers" required:"true"`
	IPAddress     string        `short:"I" long:"IP-address" description:"IP address or name" required:"true"`
	Port          int           `short:"p" long:"port" default:"80" description:"Port number"`
	Method        string        `short:"j" long:"method" default:"GET" description:"Set HTTP Method"`
	URI           string        `short:"u" long:"uri" default:"/" description:"URI to request"`
	Expect        int           `short:"e" long:"expect" default:"200" description:"Expected HTTP response status"`
	UserAgent     string        `short:"A" long:"useragent" default:"check_http" description:"UserAgent to be sent"`
	Authorization string        `short:"a" long:"authorization" description:"username:password on sites with basic authentication"`
	SNI           bool          `long:"sni" description:"enable SNI"`
	TCP4          bool          `short:"4" description:"use tcp4 only"`
	Version       bool          `short:"v" long:"version" description:"Show version"`
}

type connError struct {
	e error
}

func (ce *connError) Error() string { return ce.e.Error() }

func makeTransport() http.RoundTripper {
	baseDialFunc := (&net.Dialer{
		Timeout:   opts.Timeout,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext
	tcpMode := "tcp"
	if opts.TCP4 {
		tcpMode = "tcp4"
	}
	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := baseDialFunc(ctx, tcpMode, addr)
		if err == nil {
			return conn, nil
		}
		if err != context.Canceled {
			return nil, &connError{err}
		}
		return nil, err
	}
	return &http.Transport{
		// inherited http.DefaultTransport
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialFunc,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   opts.Timeout,
		ExpectContinueTimeout: 1 * time.Second,
		// self-customized values
		ResponseHeaderTimeout: opts.Timeout,
		TLSClientConfig: &tls.Config{
			ServerName: host,
		},
		ForceAttemptHTTP2: true,
	}

}

func main() {
	os.Exit(_main())
}

var opts commandOpts

func _main() int {
	opts = commandOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		os.Exit(1)
	}
	return 1
}
