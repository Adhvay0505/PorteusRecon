package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
)

// Check if a port is open
func checkPort(host string, port int, wg *sync.WaitGroup) {
	defer wg.Done()

	address := fmt.Sprintf("%s:%d", host, port)
	// Attempt to connect to the port (this will block until either connected or timeout)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		// If there's an error, the port is closed or unreachable
		fmt.Printf("Port %d is closed\n", port)
		return
	}
	defer conn.Close()
	// Port is open
	fmt.Printf("Port %d is open\n", port)
}

func main() {
	// Parse command-line arguments
	hostPtr := flag.String("host", "", "The host IP or domain to scan")
	startPortPtr := flag.Int("start", 1, "The starting port to scan")
	endPortPtr := flag.Int("end", 1024, "The ending port to scan")
	flag.Parse()

	// Check if the host argument is provided
	if *hostPtr == "" {
		fmt.Println("Error: Host is required.")
		flag.Usage()
		os.Exit(1)
	}

	// Validate the port range
	if *startPortPtr <= 0 || *endPortPtr <= 0 || *startPortPtr > *endPortPtr {
		fmt.Println("Error: Invalid port range.")
		flag.Usage()
		os.Exit(1)
	}

	var wg sync.WaitGroup

	// Iterate over the port range and check each port
	for port := *startPortPtr; port <= *endPortPtr; port++ {
		wg.Add(1)
		go checkPort(*hostPtr, port, &wg)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

