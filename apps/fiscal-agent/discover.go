package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
	"time"
)

type discoveredPrinter struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Addr string `json:"addr"`
}

func localScanSubnets() ([]*net.IPNet, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var nets []*net.IPNet
	seen := make(map[string]struct{})
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			ones, bits := ipNet.Mask.Size()
			if bits != 32 {
				continue
			}
			// Cap scan size: treat wider than /24 as /24 around the interface IP.
			if ones < 24 {
				ip := ipNet.IP.To4()
				mask := net.CIDRMask(24, 32)
				ipNet = &net.IPNet{IP: ip.Mask(mask), Mask: mask}
			}
			key := ipNet.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			nets = append(nets, ipNet)
		}
	}
	return nets, nil
}

func hostsInSubnet(ipNet *net.IPNet) []net.IP {
	ip := ipNet.IP.To4()
	if ip == nil {
		return nil
	}
	mask := ipNet.Mask
	network := ip.Mask(mask)
	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = network[i] | ^mask[i]
	}

	var hosts []net.IP
	for d := 1; d <= 254; d++ {
		candidate := net.IPv4(network[0], network[1], network[2], byte(d))
		if candidate.Equal(network) || candidate.Equal(broadcast) {
			continue
		}
		if !ipNet.Contains(candidate) {
			continue
		}
		hosts = append(hosts, candidate)
	}
	return hosts
}

func probeTCP9100(ip net.IP, timeout time.Duration) bool {
	addr := fmt.Sprintf("%s:9100", ip.String())
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func discoverPrinters9100(timeout time.Duration, workers int) ([]discoveredPrinter, error) {
	subnets, err := localScanSubnets()
	if err != nil {
		return nil, err
	}
	if len(subnets) == 0 {
		return nil, fmt.Errorf("no active IPv4 LAN interfaces found")
	}

	var targets []net.IP
	for _, sn := range subnets {
		targets = append(targets, hostsInSubnet(sn)...)
	}
	if len(targets) == 0 {
		return nil, nil
	}

	jobs := make(chan net.IP, workers)
	var mu sync.Mutex
	var found []discoveredPrinter
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				if !probeTCP9100(ip, timeout) {
					continue
				}
				host := ip.String()
				mu.Lock()
				found = append(found, discoveredPrinter{Host: host, Port: 9100, Addr: host + ":9100"})
				mu.Unlock()
			}
		}()
	}

	for _, ip := range targets {
		jobs <- ip
	}
	close(jobs)
	wg.Wait()

	sort.Slice(found, func(i, j int) bool {
		return found[i].Host < found[j].Host
	})
	return found, nil
}

func runDiscover(args []string) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	timeoutMs := fs.Int("timeout-ms", 400, "TCP connect timeout per host (ms)")
	workers := fs.Int("workers", 64, "Concurrent probes")
	asJSON := fs.Bool("json", false, "Print JSON instead of human-readable lines")
	_ = fs.Parse(args)

	timeout := time.Duration(*timeoutMs) * time.Millisecond
	tcpList, winList, err := discoverAllPrinters(timeout, *workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discover: %v\n", err)
		os.Exit(1)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"tcp": tcpList, "winspool": winList})
		return
	}

	fmt.Println("LAN (TCP port 9100):")
	if len(tcpList) == 0 {
		fmt.Println("  (none — check cable, power, same network)")
	} else {
		for _, p := range tcpList {
			fmt.Printf("  %s\n", p.Addr)
		}
	}
	fmt.Println()
	fmt.Println("USB / Windows printer queues:")
	if len(winList) == 0 {
		fmt.Println("  (none — install UNYKA UK56009 driver on Windows)")
	} else {
		for _, p := range winList {
			fmt.Printf("  %s\n", p.Addr)
		}
	}
	fmt.Println()
	fmt.Println("Add to config.json, for example:")
	fmt.Println(`  "default_printer": "tcp:192.168.1.50:9100",`)
	fmt.Println(`  "default_printer": "winspool:UK56009",`)
	fmt.Println(`  "station_printers": {`)
	fmt.Println(`    "<print_station_uuid>": "tcp:192.168.1.51:9100",`)
	fmt.Println(`    "<print_station_uuid>": "winspool:Bar Printer"`)
	fmt.Println(`  }`)
	fmt.Println("Or run MesaPrintAgent setup for a guided page.")
	fmt.Println("Station UUIDs: dashboard → 出品档口")
}
