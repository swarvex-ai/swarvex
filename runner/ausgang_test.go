package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOeffentlicheAdresse(t *testing.T) {
	for adresse, erwartet := range map[string]bool{
		"1.1.1.1": true, "140.82.112.3": true, "2606:4700:4700::1111": true,
		"10.0.0.1": false, "192.168.1.103": false, "172.16.5.5": false, "127.0.0.1": false, "169.254.169.254": false,
		"100.64.1.2": false, "0.0.0.0": false, "::1": false, "fd00::1": false, "fe80::1": false, "::ffff:192.168.1.1": false,
		"64:ff9b::c0a8:0101": false, "224.0.0.1": false, "198.18.0.1": false,
	} {
		if got := oeffentlicheAdresse(net.ParseIP(adresse)); got != erwartet {
			t.Errorf("%s: got %v, want %v", adresse, got, erwartet)
		}
	}
}

func TestAusgangRegeln(t *testing.T) {
	r, err := ausgangRegelnLesen("public", "192.168.1.50, 10.9.0.0/16, mirror.lan")
	if err != nil {
		t.Fatal(err)
	}
	pruefe := func(host, ip string, erwartet bool) {
		t.Helper()
		if got := r.erlaubt(host, net.ParseIP(ip)); got != erwartet {
			t.Errorf("%s %s: got %v, want %v", host, ip, got, erwartet)
		}
	}
	pruefe("registry.npmjs.org", "104.16.0.1", true)
	pruefe("nas.local", "192.168.1.20", false)
	pruefe("spiegel", "192.168.1.50", true)
	pruefe("x", "10.9.3.4", true)
	pruefe("mirror.lan", "192.168.7.7", true)
	pruefe("mirror.lan", "127.0.0.1", false) // nie der Proxy selbst
	lan, _ := ausgangRegelnLesen("lan", "")
	if !lan.erlaubt("nas", net.ParseIP("192.168.1.20")) || lan.erlaubt("x", net.ParseIP("127.0.0.1")) {
		t.Fatal("lan mode: private allowed, loopback never")
	}
	if _, err := ausgangRegelnLesen("offen", ""); err == nil {
		t.Fatal("unknown mode must fail")
	}
}

// Der Proxy verbindet auf die geprüfte Adresse: ein Ziel, das auf eine private Adresse auflöst, wird abgewiesen,
// ein öffentliches durchgelassen (hier gespielt: die "öffentliche" Adresse leitet der Test auf einen lokalen Server um).
func TestAusgangProxy(t *testing.T) {
	ziel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "paket") }))
	defer ziel.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(ziel.URL, "http://"))
	a := &ausgang{regeln: ausgangRegeln{hosts: map[string]bool{}},
		aufl: func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host == "registry.example" {
				return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
			}
			return []net.IPAddr{{IP: net.ParseIP("192.168.1.1")}}, nil // router.example und alles andere
		},
		dial: func(ctx context.Context, n, addr string) (net.Conn, error) {
			if !strings.HasPrefix(addr, "93.184.216.34:") {
				t.Fatalf("dialled an unchecked address %s", addr)
			}
			return (&net.Dialer{}).DialContext(ctx, n, "127.0.0.1:"+port)
		}}
	proxy := httptest.NewServer(a)
	defer proxy.Close()
	pu, _ := url.Parse(proxy.URL)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu)}}
	r, err := client.Get("http://registry.example:" + port + "/paket.tgz")
	if err != nil {
		t.Fatal(err)
	}
	roh, _ := io.ReadAll(r.Body)
	if r.StatusCode != 200 || string(roh) != "paket" {
		t.Fatalf("public target: %d %s", r.StatusCode, roh)
	}
	r, err = client.Get("http://router.example/")
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("private target: want 403, got %d", r.StatusCode)
	}
	// CONNECT (HTTPS) zu einer privaten Adresse: abgewiesen
	verb, _ := net.Dial("tcp", pu.Host)
	defer verb.Close()
	io.WriteString(verb, "CONNECT router.example:443 HTTP/1.1\r\nHost: router.example:443\r\n\r\n")
	antwort := make([]byte, 64)
	n, _ := verb.Read(antwort)
	if !strings.Contains(string(antwort[:n]), "403") {
		t.Fatalf("CONNECT to a private address: %q", antwort[:n])
	}
	// CONNECT zu einer öffentlichen Adresse: Tunnel steht, Bytes gehen in beide Richtungen
	echo, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echo.Close()
	go func() {
		c, err := echo.Accept()
		if err == nil {
			io.Copy(c, c)
			c.Close()
		}
	}()
	_, echoPort, _ := net.SplitHostPort(echo.Addr().String())
	a.dial = func(ctx context.Context, n, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, n, "127.0.0.1:"+echoPort)
	}
	tunnel, _ := net.Dial("tcp", pu.Host)
	defer tunnel.Close()
	io.WriteString(tunnel, "CONNECT registry.example:443 HTTP/1.1\r\nHost: registry.example:443\r\n\r\n")
	kopf := make([]byte, 39)
	io.ReadFull(tunnel, kopf)
	if !strings.HasPrefix(string(kopf), "HTTP/1.1 200") {
		t.Fatalf("CONNECT to a public address: %q", kopf)
	}
	io.WriteString(tunnel, "hallo")
	echoAntwort := make([]byte, 5)
	io.ReadFull(tunnel, echoAntwort)
	if string(echoAntwort) != "hallo" {
		t.Fatalf("tunnel did not pass bytes: %q", echoAntwort)
	}
}
