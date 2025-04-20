package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
)

// Mutex for safe printing
var mu sync.Mutex

// Check if a port is open
func checkPort(host string, port int, wg *sync.WaitGroup) {
	defer wg.Done()

	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	// Attempt to connect to the port (this will block until either connected or timeout)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		// If there is an error, the port is closed or unreachable
		mu.Lock()
		fmt.Printf("Port %d is closed: %v\n", port, err)
		mu.Unlock()
		return
	}
	defer conn.Close()
	// Port is open
	mu.Lock()
	fmt.Printf("Port %d is open\n", port)
	mu.Unlock()
}

func main() {

	// Custom usage to control help output format and order
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "  -host string")
		fmt.Fprintln(os.Stderr, "    \tThe host IP or domain to scan (REQUIRED)")
		fmt.Fprintln(os.Stderr, "  -start int")
		fmt.Fprintln(os.Stderr, "    \tThe starting port to scan (default 1)")
		fmt.Fprintln(os.Stderr, "  -end int")
		fmt.Fprintln(os.Stderr, "    \tThe ending port to scan (default 1024)")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintln(os.Stderr, "  go run PorteusRecon0.go -host 192.168.1.1 -start 20 -end 80")
	}

	// Parse command-line arguments
	hostPtr := flag.String("host", "", "The host IP or domain to scan")
	startPortPtr := flag.Int("start", 1, "The starting port to scan")
	endPortPtr := flag.Int("end", 1024, "The ending port to scan")
	flag.Parse()

	// Check if the host is provided
	if *hostPtr == "" {
		fmt.Println("Error: Host is required!")
		flag.Usage()
		os.Exit(1)
	}

	// check the port range
	if *startPortPtr <= 0 || *endPortPtr <= 0 || *startPortPtr > *endPortPtr {
		fmt.Println("Error: Invalid port range.")
		flag.Usage()
		os.Exit(1)
	}

	var wg sync.WaitGroup

	// check each port
	for port := *startPortPtr; port <= *endPortPtr; port++ {
		wg.Add(1)
		go checkPort(*hostPtr, port, &wg)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}
