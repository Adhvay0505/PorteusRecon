package main

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

func makeUI(myApp fyne.App, myWindow fyne.Window, isDarkMode bool) {
	// Set the theme
	if isDarkMode {
		myApp.Settings().SetTheme(theme.DarkTheme())
	} else {
		myApp.Settings().SetTheme(theme.LightTheme())
	}

	// Declare widgets up front for scope
	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("Enter host (e.g. 127.0.0.1)")

	startPortEntry := widget.NewEntry()
	startPortEntry.SetPlaceHolder("Start Port (e.g. 1)")

	endPortEntry := widget.NewEntry()
	endPortEntry.SetPlaceHolder("End Port (e.g. 1024)")

	// Use a read-only Label within a scroll container for results (avoids SetReadOnly issue)
	resultLabel := widget.NewLabel("")
	resultLabel.Wrapping = fyne.TextWrapWord
	scrollWrapper := container.NewScroll(resultLabel)
	scrollWrapper.SetMinSize(fyne.NewSize(500, 200))

	var wg sync.WaitGroup
	resultChan := make(chan string)
	var mu sync.Mutex
	var scanButton *widget.Button

	scanButton = widget.NewButton("Start Scan", func() {
		host := hostEntry.Text
		startPort, err1 := strconv.Atoi(startPortEntry.Text)
		endPort, err2 := strconv.Atoi(endPortEntry.Text)

		if err1 != nil || err2 != nil || startPort <= 0 || endPort <= 0 || startPort > endPort {
			resultLabel.SetText("Invalid port range.")
			return
		}

		scanButton.Disable()
		resultLabel.SetText("Scanning...")
		resultChan = make(chan string)
		var results string

		for port := startPort; port <= endPort; port++ {
			wg.Add(1)
			go checkPort(host, port, resultChan, &wg)
		}

		go func() {
			go func() {
				wg.Wait()
				close(resultChan)
			}()
			for result := range resultChan {
				mu.Lock()
				results += result + "\n"
				mu.Unlock()
				// Update the result safely on the UI thread
				res := results
				fyne.CurrentApp().SendNotification(&fyne.Notification{})
				myWindow.Canvas().Refresh(scrollWrapper)
				resultLabel.SetText(res)
			}
			scanButton.Enable()
		}()
	})

	isDark := isDarkMode
	var toggleThemeButton *widget.Button

	// Initialize the toggle button with dynamic text based on current mode
	toggleText := "Dark Mode"
	if isDark {
		toggleText = "Light Mode"
	}
	toggleThemeButton = widget.NewButton(toggleText, func() {
		isDark = !isDark
		makeUI(myApp, myWindow, isDark)
	})

	topRow := container.NewHBox(
		widget.NewLabelWithStyle("Porteus Recon", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		toggleThemeButton,
	)

	content := container.NewVBox(
		topRow,
		hostEntry,
		startPortEntry,
		endPortEntry,
		scanButton,
		scrollWrapper, // Use the scroll container with the label here
	)

	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(500, 400))
	myWindow.Show()
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Porteus Recon")
	makeUI(myApp, myWindow, true)
	myApp.Run()
}

