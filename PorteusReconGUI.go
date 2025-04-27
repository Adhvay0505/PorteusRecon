package main

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var mu sync.Mutex

func checkPort(host string, port int, resultChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.Dial("tcp", address)
	if err != nil {
		resultChan <- fmt.Sprintf("Port %d is closed", port)
		return
	}
	conn.Close()
	resultChan <- fmt.Sprintf("Port %d is open", port)
}

func makeUI(myApp fyne.App, isDarkMode bool) {
	// Set theme
	if isDarkMode {
		myApp.Settings().SetTheme(theme.DarkTheme())
	} else {
		myApp.Settings().SetTheme(theme.LightTheme())
	}

	myWindow := myApp.NewWindow("Porteus Recon")

	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("Enter host (e.g. 127.0.0.1)")

	startPortEntry := widget.NewEntry()
	startPortEntry.SetPlaceHolder("Start Port (e.g. 1)")

	endPortEntry := widget.NewEntry()
	endPortEntry.SetPlaceHolder("End Port (e.g. 1024)")

	resultText := widget.NewMultiLineEntry()
	resultText.SetPlaceHolder("Scan results will appear here...")
	resultText.SetMinRowsVisible(15)

	var wg sync.WaitGroup

	scanButton := widget.NewButton("Start Scan", func() {
		host := hostEntry.Text
		startPort, err1 := strconv.Atoi(startPortEntry.Text)
		endPort, err2 := strconv.Atoi(endPortEntry.Text)

		if err1 != nil || err2 != nil || startPort <= 0 || endPort <= 0 || startPort > endPort {
			resultText.SetText("Invalid port range.")
			return
		}

		resultText.SetText("Scanning...")
		resultChan := make(chan string)

		for port := startPort; port <= endPort; port++ {
			wg.Add(1)
			go checkPort(host, port, resultChan, &wg)
		}

		go func() {
			wg.Wait()
			close(resultChan)
		}()

		go func() {
			var results string
			for result := range resultChan {
				mu.Lock()
				results += result + "\n"
				mu.Unlock()
			}
			resultText.SetText(results)
		}()
	})

	// Set the icon for the button depending on whether dark mode is enabled or not
	var toggleIcon string
	if isDarkMode {
		toggleIcon = "🌞" // Sun for light mode
	} else {
		toggleIcon = "🌙" // Moon for dark mode
	}

	toggleThemeButton := widget.NewButton(toggleIcon, func() {
		myWindow.Close()
		makeUI(myApp, !isDarkMode)
	})

	// Create a container for the top row with a 2-column grid layout to position the label and button
	topRow := container.NewGridWithColumns(2,
		widget.NewLabel("Porteus Recon"), // Label on the left
		toggleThemeButton,               // Button on the right
	)

	myWindow.SetContent(container.NewVBox(
		topRow,                           // Top row with label and button
		hostEntry,
		startPortEntry,
		endPortEntry,
		scanButton,
		resultText,
	))

	myWindow.Resize(fyne.NewSize(500, 400))	//width, height
	myWindow.Show()
}

func main() {
	myApp := app.New()
	makeUI(myApp, true) // Start in dark mode by default
	myApp.Run()
}

