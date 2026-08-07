package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/jessevdk/go-flags"
)

var version string
var commit string

const (
	OK = iota
	WARNING
	CRITICAL
	UNKNOWN
)

func (opt *Opt) verifyWaitFor() error {
	if opt.WaitFor && opt.WaitForMax == 0 {
		return fmt.Errorf("wait-for-max is required when wait-for is enabled")
	}
	return nil
}

func (opt *Opt) verifyExpectedContent() error {
	if opt.ExpectContent != "" && opt.Base64ExpectContent != "" {
		return fmt.Errorf("both string and base64-string are specified")
	}

	if opt.ExpectContent != "" {
		opt.expectByte = []byte(opt.ExpectContent)
	}

	if opt.Base64ExpectContent != "" {
		data, err := base64.StdEncoding.DecodeString(opt.Base64ExpectContent)
		if err != nil {
			return fmt.Errorf("failed decode base64-string: %w", err)
		}
		opt.expectByte = data
	}

	return nil
}

func (opt *Opt) verifyHostOptions() error {
	if opt.TCP4 && opt.TCP6 {
		return fmt.Errorf("both tcp4 and tcp6 are specified")
	}

	if opt.SNI && opt.Hostname == "" {
		return fmt.Errorf("hostname is required when using sni")
	}

	if opt.Hostname == "" && opt.IPAddress == "" {
		return fmt.Errorf("specify either hostname or ipaddress")
	}

	return nil
}

func (opt *Opt) normalizeHostAndIP() {
	if opt.Hostname == "" {
		opt.Hostname = opt.IPAddress
	}

	if opt.IPAddress == "" {
		host, _, err := net.SplitHostPort(opt.Hostname)
		if err != nil {
			opt.IPAddress = opt.Hostname
			return
		}
		opt.IPAddress = host
	}
}

func (opt *Opt) setDefaultPort() {
	if opt.Port == 0 {
		_, port, err := net.SplitHostPort(opt.Hostname)
		if err == nil {
			p, _ := strconv.Atoi(port)
			// skip error check OK
			opt.Port = p
		}
	}

	if opt.Port == 0 {
		if opt.SSL {
			opt.Port = 443
		} else {
			opt.Port = 80
		}
	}
}

func (opt *Opt) setDefaultURI() {
	if opt.URI == "" {
		opt.URI = "/"
	}
}

func (opt *Opt) verify() error {
	opt.bufferSize = uint64(opt.MaxBufferSize)

	if err := opt.verifyWaitFor(); err != nil {
		return err
	}

	if err := opt.verifyExpectedContent(); err != nil {
		return err
	}

	if err := opt.verifyHostOptions(); err != nil {
		return err
	}

	opt.normalizeHostAndIP()
	opt.setDefaultPort()
	opt.setDefaultURI()

	return nil
}

func (opt *Opt) BuildClient() *http.Client {
	transport := opt.MakeTransport()
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: opt.Timeout,
	}
	return client
}

func (opt *Opt) run() int {
	client := opt.BuildClient()

	ctx := context.Background()
	timeout := opt.Timeout + 3*time.Second
	if opt.WaitForMax > 0 {
		timeout = opt.WaitForMax
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if opt.WaitFor {
		msg, code := opt.runWaitFor(ctx, client)
		fmt.Println(msg)
		return code
	}

	msg, code := opt.runRequest(ctx, client)
	fmt.Println(msg)
	return code
}

func main() {
	os.Exit(_main())
}

func _main() int {
	opt := &Opt{}
	psr := flags.NewParser(opt, flags.HelpFlag|flags.PassDoubleDash)
	_, err := psr.Parse()
	if opt.Version {
		if commit == "" {
			commit = "dev"
		}
		fmt.Printf(
			"%s-%s\n%s/%s, %s, %s\n",
			filepath.Base(os.Args[0]),
			version,
			runtime.GOOS,
			runtime.GOARCH,
			runtime.Version(),
			commit)
		return OK
	} else if flags.WroteHelp(err) {
		fmt.Fprintf(os.Stdout, "%v\n", err)
		return OK
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return UNKNOWN
	}

	return opt.run()
}
