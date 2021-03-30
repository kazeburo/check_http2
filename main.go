package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jessevdk/go-flags"
)

var version string

const UNKNOWN = 3
const CRITICAL = 2
const WARNING = 1
const OK = 0

type commandOpts struct {
	Timeout             time.Duration `long:"timeout" default:"10s" description:"Timeout to wait for connection"`
	MaxBufferSize       string        `long:"max-buffer-size" default:"1MB" description:"Max buffer size to read response body"`
	NoDiscard           bool          `long:"no-discard" description:"raise error when the response body is larger then max-buffer-size"`
	WaitFor             bool          `long:"wait-for" description:"retry until successful when enabled"`
	WaitForInterval     time.Duration `long:"wait-for-interval" default:"2s" description:"retry interval"`
	WaitForMax          time.Duration `long:"wait-for-max" description:"time to wait for success"`
	Hostname            string        `short:"H" long:"hostname" description:"Host name using Host headers"`
	IPAddress           string        `short:"I" long:"IP-address" description:"IP address or name"`
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
	TCP4                bool          `short:"4" description:"use tcp4 only"`
	TCP6                bool          `short:"6" description:"use tcp6 only"`
	Version             bool          `short:"v" long:"version" description:"Show version"`
	bufferSize          uint64
	expectByte          []byte
}

func makeTransport(opts commandOpts) http.RoundTripper {
	baseDialFunc := (&net.Dialer{
		Timeout:   opts.Timeout,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext
	tcpMode := "tcp"
	if opts.TCP4 {
		tcpMode = "tcp4"
	}
	if opts.TCP6 {
		tcpMode = "tcp6"
	}
	dialFunc := func(ctx context.Context, _, _ string) (net.Conn, error) {
		addr := fmt.Sprintf("%s:%d", opts.IPAddress, opts.Port)
		return baseDialFunc(ctx, tcpMode, addr)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	if opts.SNI {
		host, _, err := net.SplitHostPort(opts.Hostname)
		if err != nil {
			host = opts.Hostname
		}
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}
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
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
	}
}

func buildRequest(ctx context.Context, opts commandOpts) (*http.Request, error) {
	schema := "http"
	if opts.SSL {
		schema = "https"
	}

	uri := fmt.Sprintf("%s://%s%s", schema, opts.Hostname, opts.URI)
	var b bytes.Buffer
	req, err := http.NewRequestWithContext(
		ctx,
		opts.Method,
		uri,
		&b,
	)
	if err != nil {
		return nil, err
	}
	if opts.Authorization != "" {
		a := strings.SplitN(opts.Authorization, ":", 2)
		if len(a) != 2 {
			return nil, fmt.Errorf("invalid authorization args")
		}
		req.SetBasicAuth(a[0], a[1])
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	return req, nil
}

func expectedStatusCode(opts commandOpts, status string) string {
	expects := strings.Split(opts.Expect, ",")
	for _, e := range expects {
		if strings.HasPrefix(status, e) {
			return e
		}
	}
	return ""
}

func printVersion() {
	fmt.Printf(`chocon %s
Compiler: %s %s
`,
		version,
		runtime.Compiler,
		runtime.Version())
}

type capWriter struct {
	Cap       uint64
	NoDiscard bool
	size      uint64
	buffer    []byte
}

func (w *capWriter) Write(p []byte) (int, error) {
	w.size += uint64(len(p))
	if w.size > w.Cap && w.NoDiscard {
		return 0, fmt.Errorf("Could not write body buffer. buffer is full")
	}

	if w.size > w.Cap {
		q := w.Cap - uint64(len(w.buffer))
		if q >= 0 {
			w.buffer = append(w.buffer, p[0:q-1]...)
		}
	} else {
		w.buffer = append(w.buffer, p...)
	}

	return len(p), nil
}

func (w *capWriter) Size() uint64 {
	return w.size
}

func (w *capWriter) Bytes() []byte {
	return w.buffer
}

type reqError struct {
	msg  string
	code int
}

func (e *reqError) Error() string {
	return e.msg
}

func (e *reqError) Code() int {
	return e.code
}

func request(ctx context.Context, opts commandOpts) (string, *reqError) {
	req, err := buildRequest(ctx, opts)
	if err != nil {
		return "", &reqError{
			fmt.Sprintf("Error in building request: %v", err),
			UNKNOWN,
		}
	}

	transport := makeTransport(opts)
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	start := time.Now()
	res, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return "", &reqError{
			fmt.Sprintf("Error in request: %v", err),
			UNKNOWN,
		}
	}

	defer res.Body.Close()
	b := &capWriter{
		Cap:       opts.bufferSize,
		NoDiscard: opts.NoDiscard,
	}
	_, err = io.Copy(b, res.Body)
	if err != nil {
		return "", &reqError{
			fmt.Sprintf("Error in read response: %v", err),
			UNKNOWN,
		}
	}

	var matched []string

	statusLine := fmt.Sprintf("%s %s", res.Proto, res.Status)
	if opts.Expect != "" {
		m := expectedStatusCode(opts, statusLine)
		if m == "" {
			return "", &reqError{
				fmt.Sprintf("HTTP CRITICAL - Invalid HTTP response received from host on port %d: %s", opts.Port, statusLine),
				CRITICAL,
			}
		} else {
			matched = append(matched, fmt.Sprintf(`Status line output "%s" matched "%s"`, statusLine, opts.Expect))
		}
	}

	if len(opts.expectByte) > 0 {
		if !bytes.Contains(b.Bytes(), opts.expectByte) {
			return "", &reqError{
				fmt.Sprintf(`HTTP CRITICAL - HTTP response body Not matched "%s" from host on port %d`, string(opts.expectByte), opts.Port),
				CRITICAL,
			}
		} else {
			matched = append(matched, fmt.Sprintf(`Response body matched "%s"`, opts.ExpectContent))
		}
	}

	b.Write([]byte(statusLine + "\r\n\r\n"))
	res.Header.Write(b)

	okMsg := fmt.Sprintf(`HTTP OK: %s  - %d bytes in %.3f second response time | time=%fs;;;0.000000 size=%dB;;;0`, strings.Join(matched, ", "), b.Size(), duration.Seconds(), duration.Seconds(), b.Size())
	return okMsg, nil
}

func main() {
	os.Exit(_main())
}

func _main() int {
	opts := commandOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		os.Exit(UNKNOWN)
	}

	if opts.Version {
		printVersion()
		return OK
	}

	bufferSize, err := humanize.ParseBytes(opts.MaxBufferSize)
	if err != nil {
		fmt.Printf("Could not parse max-buffer-size: %v\n", err)
		return UNKNOWN
	}
	opts.bufferSize = bufferSize

	if opts.WaitFor && opts.WaitForMax == 0 {
		fmt.Printf("wait-for-max is required when wait-for is enabled\n")
		return UNKNOWN
	}

	if opts.ExpectContent != "" && opts.Base64ExpectContent != "" {
		fmt.Printf("Both string and base64-string are specified\n")
		return UNKNOWN
	}

	if opts.ExpectContent != "" {
		opts.expectByte = []byte(opts.ExpectContent)
	}
	if opts.Base64ExpectContent != "" {
		data, err := base64.StdEncoding.DecodeString(opts.Base64ExpectContent)
		if err != nil {
			fmt.Printf("Failed decode base64-string: %v\n", err)
			return UNKNOWN
		}
		opts.expectByte = data
	}

	if opts.TCP4 && opts.TCP6 {
		fmt.Printf("Both tcp4 and tcp6 are specified\n")
		return UNKNOWN
	}

	if opts.SNI && opts.Hostname == "" {
		fmt.Printf("hostname is required when use sni\n")
		return UNKNOWN
	}

	if opts.Hostname == "" && opts.IPAddress == "" {
		fmt.Printf("Specify either hostname or ipaddress\n")
		return UNKNOWN
	}

	if opts.Hostname == "" {
		opts.Hostname = opts.IPAddress
	}

	if opts.IPAddress == "" {
		host, _, err := net.SplitHostPort(opts.Hostname)
		if err != nil {
			opts.IPAddress = opts.Hostname
		} else {
			opts.IPAddress = host
		}
	}

	if opts.Port == 0 {
		_, port, err := net.SplitHostPort(opts.Hostname)
		if err == nil {
			p, _ := strconv.Atoi(port)
			// skip error check OK
			opts.Port = p
		}
	}

	if opts.Port == 0 {
		if opts.SSL {
			opts.Port = 443
		} else {
			opts.Port = 80
		}
	}

	if opts.URI == "" {
		opts.URI = "/"
	}

	ctx := context.Background()
	timeout := opts.Timeout + 3*time.Second
	if opts.WaitForMax > 0 {
		timeout = opts.WaitForMax
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if opts.WaitFor {
		for ctx.Err() == nil {
			okMsg, reqErr := request(ctx, opts)
			if reqErr == nil {
				fmt.Println(okMsg)
				return OK
			}
			log.Printf(reqErr.Error())
			select {
			case <-ctx.Done():
			case <-time.After(opts.WaitForInterval):
			}
		}
		fmt.Printf("Give up waiting for success\n")
		return UNKNOWN
	}

	okMsg, reqErr := request(ctx, opts)
	if reqErr != nil {
		fmt.Println(reqErr.Error())
		return reqErr.Code()
	}

	fmt.Println(okMsg)
	return OK
}
