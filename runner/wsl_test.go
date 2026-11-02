package main

import "testing"

func TestStandardGateway(t *testing.T) {
	// /proc/net/route einer WSL-VM im NAT-Modus: Gateway 172.24.16.1 (Bytes in Little-Endian)
	tabelle := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t011018AC\t0003\t0\t0\t0\t00000000\t0\t0\t0\n" +
		"eth0\t001018AC\t00000000\t0001\t0\t0\t0\t00F0FFFF\t0\t0\t0\n"
	if g := standardGateway(tabelle); g != "172.24.16.1" {
		t.Fatalf("gateway: got %q", g)
	}
	if g := standardGateway("Iface\tDestination\tGateway\n"); g != "" {
		t.Fatalf("no default route: got %q", g)
	}
}
