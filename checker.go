package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// HumanBytes is a custom type to handle human-readable byte inputs
type HumanBytes uint64

// UnmarshalFlag parses human inputs like "10MB" or "2GiB" using go-humanize
func (hb *HumanBytes) UnmarshalFlag(value string) error {
	n, err := humanize.ParseBytes(value)
	if err != nil {
		return err
	}
	*hb = HumanBytes(n)
	return nil
}

type Opt struct {
	Timeout       time.Duration `long:"timeout" default:"10s" description:"Timeout to wait for connection"`
	MaxBufferSize HumanBytes    `long:"max-buffer-size" default:"1MB" description:"Max buffer size to read response body"`
	NoDiscard     bool          `long:"no-discard" description:"raise error when the response body is larger then max-buffer-size"`

	Consecutive int           `long:"consecutive" default:"1" description:"number of consecutive successful requests required"`
	Interim     time.Duration `long:"interim" default:"1s" description:"interval time after successful request for consecutive mode"`

	WaitFor             bool          `long:"wait-for" description:"retry until successful when enabled"`
	WaitForInterval     time.Duration `long:"wait-for-interval" default:"2s" description:"retry interval"`
	WaitForMax          time.Duration `long:"wait-for-max" description:"time to wait for success"`
	Hostname            string        `short:"H" long:"hostname" description:"Host name using Host headers"`
	IPAddress           string        `short:"I" long:"IP-address" description:"IP address or Host name"`
	Port                int           `short:"p" long:"port" description:"Port number"`
	Method              string        `short:"j" long:"method" default:"GET" description:"Set HTTP Method"`
	URI                 string        `short:"u" long:"uri" default:"/" description:"URI to request"`
	Expect              string        `short:"e" long:"expect" default:"HTTP/1.,HTTP/2." description:"Comma-delimited list of expected HTTP response status"`
	ExpectContent       string        `short:"s" long:"string" description:"String to expect in the content"`
	Base64ExpectContent string        `long:"base64-string" description:"Base64 Encoded string to expect the content"`
	UserAgent           string        `short:"A" long:"useragent" default:"check_http" description:"UserAgent to be sent"`
	Authorization       string        `short:"a" long:"authorization" description:"username:password on sites with basic authentication"`
	SSL                 bool          `short:"S" long:"ssl" description:"use https"`
	SNI                 bool          `long:"sni" description:"enable SNI"`
	TLSMaxVersion       string        `long:"tls-max" description:"maximum supported TLS version" choice:"1.0" choice:"1.1" choice:"1.2" choice:"1.3"`
	TCP4                bool          `short:"4" description:"use tcp4 only"`
	TCP6                bool          `short:"6" description:"use tcp6 only"`
	VerifySSL           bool          `long:"verify-ssl" description:"verify SSL certificate"`
	Version             bool          `short:"v" long:"version" description:"Show version"`
	bufferSize          uint64
	expectByte          []byte
}

type RequestError struct {
	msg  string
	code int
}

func (e *RequestError) Error() string {
	return e.msg
}

func (e *RequestError) Code() int {
	return e.code
}

func (opt *Opt) MakeTransport() http.RoundTripper {
	baseDialFunc := (&net.Dialer{
		Timeout:   opt.Timeout,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext
	tcpMode := "tcp"
	if opt.TCP4 {
		tcpMode = "tcp4"
	}
	if opt.TCP6 {
		tcpMode = "tcp6"
	}
	dialFunc := func(ctx context.Context, _, _ string) (net.Conn, error) {
		addr := net.JoinHostPort(opt.IPAddress, fmt.Sprintf("%d", opt.Port))
		return baseDialFunc(ctx, tcpMode, addr)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: !opt.VerifySSL,
	}
	if opt.SNI {
		host, _, err := net.SplitHostPort(opt.Hostname)
		if err != nil {
			host = opt.Hostname
		}
		tlsConfig.ServerName = host
	}

	if opt.TLSMaxVersion != "" {
		switch opt.TLSMaxVersion {
		case "1.0":
			tlsConfig.MinVersion = tls.VersionTLS10
			tlsConfig.MaxVersion = tls.VersionTLS10
		case "1.1":
			tlsConfig.MinVersion = tls.VersionTLS11
			tlsConfig.MaxVersion = tls.VersionTLS11
		case "1.2":
			tlsConfig.MaxVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MaxVersion = tls.VersionTLS13
		}
	}

	return &http.Transport{
		// inherited http.DefaultTransport
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialFunc,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   opt.Timeout,
		ExpectContinueTimeout: 1 * time.Second,
		// self-customized values
		ResponseHeaderTimeout: opt.Timeout,
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
	}
}

func (opt *Opt) BuildRequest(ctx context.Context) (*http.Request, error) {
	schema := "http"
	if opt.SSL {
		schema = "https"
	}

	uri := fmt.Sprintf("%s://%s%s", schema, opt.Hostname, opt.URI)
	var b bytes.Buffer
	req, err := http.NewRequestWithContext(
		ctx,
		opt.Method,
		uri,
		&b,
	)
	if err != nil {
		return nil, err
	}
	if opt.Authorization != "" {
		a := strings.SplitN(opt.Authorization, ":", 2)
		if len(a) != 2 {
			return nil, fmt.Errorf("invalid authorization args")
		}
		req.SetBasicAuth(a[0], a[1])
	}
	req.Header.Set("User-Agent", opt.UserAgent)
	return req, nil
}

func (opt *Opt) ExpectedStatusCode(status string) string {
	expects := strings.SplitSeq(opt.Expect, ",")
	for e := range expects {
		if strings.HasPrefix(status, e) {
			return e
		}
	}
	return ""
}

func (opt *Opt) Request(ctx context.Context, client *http.Client) (string, *RequestError) {
	req, err := opt.BuildRequest(ctx)
	if err != nil {
		return "", &RequestError{
			fmt.Sprintf("Error in building request: %v", err),
			UNKNOWN,
		}
	}

	start := time.Now()
	res, err := client.Do(req)

	if err != nil {
		return "", &RequestError{
			fmt.Sprintf("HTTP CRITICAL - Error in request: %v", err),
			CRITICAL,
		}
	}

	b := &CapWriter{
		Cap:       opt.bufferSize,
		NoDiscard: opt.NoDiscard,
	}
	defer res.Body.Close()
	_, err = io.Copy(b, res.Body)
	if err != nil {
		return "", &RequestError{
			fmt.Sprintf("HTTP CRITICAL - Error in read response: %v", err),
			CRITICAL,
		}
	}

	duration := time.Since(start)
	var matched []string

	statusLine := fmt.Sprintf("%s %s", res.Proto, res.Status)
	if opt.Expect != "" {
		m := opt.ExpectedStatusCode(statusLine)
		if m == "" {
			return "", &RequestError{
				fmt.Sprintf("HTTP CRITICAL - Invalid HTTP response received from host on port %d: %s", opt.Port, statusLine),
				CRITICAL,
			}
		} else {
			matched = append(matched, fmt.Sprintf(`Status line output "%s" matched "%s"`, statusLine, opt.Expect))
		}
	}

	if len(opt.expectByte) > 0 {
		if !bytes.Contains(b.Bytes(), opt.expectByte) {
			return "", &RequestError{
				fmt.Sprintf(`HTTP CRITICAL - HTTP response body Not matched %q from host on port %d`, string(opt.expectByte), opt.Port),
				CRITICAL,
			}
		} else {
			matched = append(matched, fmt.Sprintf(`Response body matched %q`, string(opt.expectByte)))
		}
	}

	_, _ = b.Write([]byte(statusLine + "\r\n\r\n"))
	_ = res.Header.Write(b)

	okMsg := fmt.Sprintf(`HTTP OK: %s  - %d bytes in %.3f second response time | time=%fs;;;0.000000 size=%dB;;;0`, strings.Join(matched, ", "), b.Size(), duration.Seconds(), duration.Seconds(), b.Size())
	return okMsg, nil
}
