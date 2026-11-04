package main

// Ausgangs-Proxy für das Einrichten (Abhängigkeiten laden): Der Einrichtungs-Container hängt nur am internen Netz und
// erreicht das Internet über diesen Proxy (HTTP_PROXY/HTTPS_PROXY). Er löst jedes Ziel selbst auf, lehnt private,
// lokale und reservierte Adressen ab (Heimnetz, Router, NAS, der eigene Rechner) und verbindet auf genau die geprüfte
// Adresse, damit eine zweite DNS-Antwort nichts umleiten kann. Läuft im Image swarvex-proxy: `swarvex proxy egress`.
//
//	SWARVEX_EGRESS_MODE   public (Standard): nur öffentliche Adressen; lan: auch private (nicht Loopback)
//	SWARVEX_EGRESS_ALLOW  Ausnahmen trotz „public“, durch Komma getrennt: IP, CIDR oder Hostname (z. B. ein Paketspiegel)

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"sync"
	"time"
)

var reserviert = func() []*net.IPNet {
	var r []*net.IPNet
	for _, c := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24",
		"203.0.113.0/24", "240.0.0.0/4", "64:ff9b::/96", "64:ff9b:1::/48", "2001:db8::/32", "fec0::/10", "100::/64"} {
		_, n, _ := net.ParseCIDR(c)
		r = append(r, n)
	}
	return r
}()

// oeffentlicheAdresse: nicht privat, nicht lokal, nicht reserviert (auch nicht als IPv4 in IPv6 verpackt).
func oeffentlicheAdresse(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, n := range reserviert {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

type ausgangRegeln struct {
	lan      bool
	ausnahme []*net.IPNet
	hosts    map[string]bool
}

func ausgangRegelnLesen(modus, erlaubt string) (ausgangRegeln, error) {
	r := ausgangRegeln{hosts: map[string]bool{}}
	switch modus {
	case "", "public":
	case "lan":
		r.lan = true
	default:
		return r, fmt.Errorf("SWARVEX_EGRESS_MODE: public or lan, not %q", modus)
	}
	for _, e := range strings.Split(erlaubt, ",") {
		e = strings.TrimSpace(e)
		switch {
		case e == "":
		case strings.Contains(e, "/"):
			_, n, err := net.ParseCIDR(e)
			if err != nil {
				return r, fmt.Errorf("SWARVEX_EGRESS_ALLOW: %v", err)
			}
			r.ausnahme = append(r.ausnahme, n)
		case net.ParseIP(e) != nil:
			ip := net.ParseIP(e)
			r.ausnahme = append(r.ausnahme, &net.IPNet{IP: ip, Mask: net.CIDRMask(len(ip)*8, len(ip)*8)})
		default:
			r.hosts[strings.ToLower(e)] = true
		}
	}
	return r, nil
}

// erlaubt: darf dieser Host über diese Adresse erreicht werden?
func (r ausgangRegeln) erlaubt(host string, ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() {
		return false // nie: das wäre der Proxy selbst
	}
	if oeffentlicheAdresse(ip) || r.hosts[strings.ToLower(host)] {
		return true
	}
	for _, n := range r.ausnahme {
		if n.Contains(ip) {
			return true
		}
	}
	return r.lan && !ip.IsMulticast()
}

type ausgang struct {
	regeln   ausgangRegeln
	aufl     func(ctx context.Context, host string) ([]net.IPAddr, error)
	dial     func(ctx context.Context, network, addr string) (net.Conn, error)
	gemeldet sync.Map
}

// ziel löst host auf, prüft jede Adresse und liefert die erste erlaubte als „ip:port“ (verbunden wird genau dorthin).
func (a *ausgang) ziel(ctx context.Context, hostport string) (string, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", err
	}
	var ips []net.IPAddr
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IPAddr{{IP: ip}}
	} else if ips, err = a.aufl(ctx, host); err != nil {
		return "", err
	}
	for _, ip := range ips {
		if a.regeln.erlaubt(host, ip.IP) {
			if _, schon := a.gemeldet.LoadOrStore(host, true); !schon {
				ereignis("egress", map[string]any{"host": host})
			}
			return net.JoinHostPort(ip.IP.String(), port), nil
		}
	}
	ereignis("egress_blocked", map[string]any{"host": host, "reason": "private, local or reserved address"})
	return "", fmt.Errorf("blocked: %s resolves only to private, local or reserved addresses", host)
}

func (a *ausgang) verbinden(ctx context.Context, hostport string) (net.Conn, error) {
	z, err := a.ziel(ctx, hostport)
	if err != nil {
		return nil, err
	}
	return a.dial(ctx, "tcp", z)
}

func (a *ausgang) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, ende := context.WithTimeout(r.Context(), 30*time.Second)
	defer ende()
	if r.Method == http.MethodConnect {
		ziel, err := a.verbinden(ctx, r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			ziel.Close()
			http.Error(w, "no hijack", http.StatusInternalServerError)
			return
		}
		// Erst die Verbindung übernehmen, dann die Antwort selbst schreiben (ein WriteHeader davor ginge verloren)
		quelle, puffer, err := hj.Hijack()
		if err != nil {
			ziel.Close()
			return
		}
		if _, err := quelle.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			ziel.Close()
			quelle.Close()
			return
		}
		go func() {
			if puffer.Reader.Buffered() > 0 {
				io.CopyN(ziel, puffer, int64(puffer.Reader.Buffered()))
			}
			io.Copy(ziel, quelle)
			ziel.Close()
		}()
		io.Copy(quelle, ziel)
		quelle.Close()
		return
	}
	if !r.URL.IsAbs() || r.URL.Scheme != "http" {
		http.Error(w, "this is a forward proxy: send absolute http:// URLs or use CONNECT", http.StatusBadRequest)
		return
	}
	hp := r.URL.Host
	if r.URL.Port() == "" {
		hp = net.JoinHostPort(r.URL.Hostname(), "80")
	}
	if _, err := a.ziel(ctx, hp); err != nil { // vorab prüfen, für eine verständliche Antwort
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	(&httputil.ReverseProxy{
		Rewrite:   func(p *httputil.ProxyRequest) { p.Out.URL = r.URL; p.Out.Host = r.URL.Host },
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) { return a.verbinden(ctx, addr) }},
	}).ServeHTTP(w, r)
}

func ausgangStarten() {
	regeln, err := ausgangRegelnLesen(os.Getenv("SWARVEX_EGRESS_MODE"), os.Getenv("SWARVEX_EGRESS_ALLOW"))
	if err != nil {
		log.Fatal("egress: " + err.Error())
	}
	d := &net.Dialer{Timeout: 15 * time.Second}
	a := &ausgang{regeln: regeln, aufl: net.DefaultResolver.LookupIPAddr, dial: d.DialContext}
	listen := envOder("SWARVEX_LISTEN", ":3128")
	log.Printf("egress: listening on %s (lan %v, %d exceptions)", listen, regeln.lan, len(regeln.ausnahme)+len(regeln.hosts))
	log.Fatal(http.ListenAndServe(listen, a))
}
