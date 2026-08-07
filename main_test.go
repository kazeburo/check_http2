package main

import "testing"

func TestVerifyBufferSize(t *testing.T) {
	opt := Opt{Hostname: "example.com", MaxBufferSize: HumanBytes(2048)}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if opt.bufferSize != 2048 {
		t.Fatalf("bufferSize = %d, want 2048", opt.bufferSize)
	}
}

func TestVerifyWaitForWithoutMax(t *testing.T) {
	opt := Opt{WaitFor: true, Hostname: "example.com"}
	if err := opt.verify(); err == nil {
		t.Fatal("verify() error = nil, want error")
	}
}

func TestVerifyExpectContentSetsExpectByte(t *testing.T) {
	opt := Opt{Hostname: "example.com", ExpectContent: "hello"}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if string(opt.expectByte) != "hello" {
		t.Fatalf("expectByte = %q, want %q", opt.expectByte, "hello")
	}
}

func TestVerifyBase64ExpectContent(t *testing.T) {
	opt := Opt{Hostname: "example.com", Base64ExpectContent: "aGVsbG8="}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if string(opt.expectByte) != "hello" {
		t.Fatalf("expectByte = %q, want %q", opt.expectByte, "hello")
	}
}

func TestVerifyBase64ExpectContentInvalid(t *testing.T) {
	opt := Opt{Hostname: "example.com", Base64ExpectContent: "!!!"}
	if err := opt.verify(); err == nil {
		t.Fatal("verify() error = nil, want error")
	}
}

func TestVerifyBothExpectContents(t *testing.T) {
	opt := Opt{Hostname: "example.com", ExpectContent: "plain", Base64ExpectContent: "aGVsbG8="}
	if err := opt.verify(); err == nil {
		t.Fatal("verify() error = nil, want error")
	}
}

func TestVerifyBothTCPModes(t *testing.T) {
	opt := Opt{Hostname: "example.com", TCP4: true, TCP6: true}
	if err := opt.verify(); err == nil {
		t.Fatal("verify() error = nil, want error")
	}
}

func TestVerifySNIRequiresHostname(t *testing.T) {
	opt := Opt{SNI: true, IPAddress: "127.0.0.1"}
	if err := opt.verify(); err == nil {
		t.Fatal("verify() error = nil, want error")
	}
}

func TestVerifyNoHost(t *testing.T) {
	opt := Opt{}
	if err := opt.verify(); err == nil {
		t.Fatal("verify() error = nil, want error")
	}
}

func TestVerifyDefaults(t *testing.T) {
	opt := Opt{Hostname: "example.com"}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if opt.URI != "/" {
		t.Fatalf("URI = %q, want %q", opt.URI, "/")
	}
	if opt.Port != 80 {
		t.Fatalf("Port = %d, want 80", opt.Port)
	}
	if opt.IPAddress != "example.com" {
		t.Fatalf("IPAddress = %q, want %q", opt.IPAddress, "example.com")
	}
}

func TestVerifyDefaultPortFromHostname(t *testing.T) {
	opt := Opt{Hostname: "example.com:8080"}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if opt.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", opt.Port)
	}
	if opt.IPAddress != "example.com" {
		t.Fatalf("IPAddress = %q, want %q", opt.IPAddress, "example.com")
	}
}

func TestVerifySSLDefaultPort(t *testing.T) {
	opt := Opt{Hostname: "example.com", SSL: true}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if opt.Port != 443 {
		t.Fatalf("Port = %d, want 443", opt.Port)
	}
}

func TestVerifyExplicitPortOverridesDefault(t *testing.T) {
	opt := Opt{Hostname: "example.com:8443", Port: 9443, SSL: true}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if opt.Port != 9443 {
		t.Fatalf("Port = %d, want 9443", opt.Port)
	}
}

func TestVerifyIPAddressFallback(t *testing.T) {
	opt := Opt{IPAddress: "192.0.2.1"}
	if err := opt.verify(); err != nil {
		t.Fatalf("verify() error = %v", err)
	}
	if opt.Hostname != "192.0.2.1" {
		t.Fatalf("Hostname = %q, want %q", opt.Hostname, "192.0.2.1")
	}
}
