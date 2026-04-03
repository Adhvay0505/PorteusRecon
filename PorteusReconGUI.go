package main

import (
	"context"
	_ "embed"
	"fmt"
	"image/color"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

//go:embed assets/fonts/jetbrainsmono/JetBrainsMono-Regular.ttf
var jetBrainsMonoRegular []byte

//go:embed assets/fonts/jetbrainsmono/JetBrainsMono-Bold.ttf
var jetBrainsMonoBold []byte

//go:embed assets/fonts/jetbrainsmono/JetBrainsMono-Italic.ttf
var jetBrainsMonoItalic []byte

//go:embed assets/fonts/jetbrainsmono/JetBrainsMono-BoldItalic.ttf
var jetBrainsMonoBoldItalic []byte

type scanResult struct {
	port int
	open bool
	err  error
}

type porteusTheme struct {
	base           fyne.Theme
	regularFont    fyne.Resource
	boldFont       fyne.Resource
	italicFont     fyne.Resource
	boldItalicFont fyne.Resource
}

func newPorteusTheme(base fyne.Theme) fyne.Theme {
	return &porteusTheme{
		base:           base,
		regularFont:    fyne.NewStaticResource("JetBrainsMono-Regular.ttf", jetBrainsMonoRegular),
		boldFont:       fyne.NewStaticResource("JetBrainsMono-Bold.ttf", jetBrainsMonoBold),
		italicFont:     fyne.NewStaticResource("JetBrainsMono-Italic.ttf", jetBrainsMonoItalic),
		boldItalicFont: fyne.NewStaticResource("JetBrainsMono-BoldItalic.ttf", jetBrainsMonoBoldItalic),
	}
}

func (t *porteusTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return t.base.Color(name, variant)
}

func (t *porteusTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Bold && style.Italic {
		return t.boldItalicFont
	}
	if style.Bold {
		return t.boldFont
	}
	if style.Italic {
		return t.italicFont
	}
	return t.regularFont
}

func (t *porteusTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *porteusTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}

func scanPort(ctx context.Context, host string, port int, timeout time.Duration) scanResult {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: timeout}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return scanResult{port: port, err: err}
	}
	_ = conn.Close()

	return scanResult{port: port, open: true}
}

func parsePositiveInt(value string, field string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive number", field)
	}
	return parsed, nil
}

func setTheme(myApp fyne.App, dark bool, toggleButton *widget.Button) {
	if dark {
		myApp.Settings().SetTheme(newPorteusTheme(theme.DarkTheme()))
		toggleButton.SetText("☀")
		return
	}

	myApp.Settings().SetTheme(newPorteusTheme(theme.LightTheme()))
	toggleButton.SetText("☾")
}

func makeFieldBlock(label string, input fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		input,
	)
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Porteus Recon")
	myWindow.Resize(fyne.NewSize(860, 620))

	isDark := true

	title := widget.NewLabelWithStyle("Porteus Recon", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Graphical Port Scanner utility")

	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("127.0.0.1 or scanme.nmap.org")
	hostEntry.SetText("127.0.0.1")

	startPortEntry := widget.NewEntry()
	startPortEntry.SetPlaceHolder("1")
	startPortEntry.SetText("1")

	endPortEntry := widget.NewEntry()
	endPortEntry.SetPlaceHolder("1024")
	endPortEntry.SetText("1024")

	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetPlaceHolder("250")
	timeoutEntry.SetText("250")

	concurrencyEntry := widget.NewEntry()
	concurrencyEntry.SetPlaceHolder("200")
	concurrencyEntry.SetText("200")

	showClosedCheck := widget.NewCheck("Show closed ports in results", nil)

	statusLabel := widget.NewLabel("Ready")
	statusLabel.Wrapping = fyne.TextWrapWord
	summaryLabel := widget.NewLabel("No scan has been run yet.")
	summaryLabel.Wrapping = fyne.TextWrapWord

	resultsOutput := widget.NewMultiLineEntry()
	resultsOutput.SetPlaceHolder("Open ports and scan messages will appear here.")
	resultsOutput.Wrapping = fyne.TextWrapWord
	resultsOutput.Disable()

	resultsScroll := container.NewScroll(resultsOutput)
	resultsScroll.SetMinSize(fyne.NewSize(640, 300))

	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	clearButton := widget.NewButton("Clear Results", func() {
		resultsOutput.SetText("")
		statusLabel.SetText("Ready")
		summaryLabel.SetText("Results cleared.")
	})
	clearButton.Importance = widget.MediumImportance

	cancelButton := widget.NewButton("Cancel Scan", nil)
	cancelButton.Importance = widget.DangerImportance
	cancelButton.Disable()

	themeButton := widget.NewButton("", nil)
	themeButton.Importance = widget.LowImportance
	setTheme(myApp, isDark, themeButton)

	var (
		cancelMu   sync.Mutex
		cancelScan context.CancelFunc
		scanID     int
	)

	var startButton *widget.Button
	startButton = widget.NewButton("Start Scan", func() {
		host := strings.TrimSpace(hostEntry.Text)
		if host == "" {
			statusLabel.SetText("Host is required.")
			return
		}

		startPort, err := parsePositiveInt(startPortEntry.Text, "start port")
		if err != nil {
			statusLabel.SetText(err.Error())
			return
		}

		endPort, err := parsePositiveInt(endPortEntry.Text, "end port")
		if err != nil {
			statusLabel.SetText(err.Error())
			return
		}

		timeoutMS, err := parsePositiveInt(timeoutEntry.Text, "timeout")
		if err != nil {
			statusLabel.SetText(err.Error())
			return
		}

		concurrency, err := parsePositiveInt(concurrencyEntry.Text, "concurrency")
		if err != nil {
			statusLabel.SetText(err.Error())
			return
		}

		if startPort > endPort {
			statusLabel.SetText("Start port must be less than or equal to end port.")
			return
		}

		if concurrency > 4096 {
			statusLabel.SetText("Concurrency is capped at 4096.")
			return
		}

		cancelMu.Lock()
		if cancelScan != nil {
			cancelScan()
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancelScan = cancel
		scanID++
		currentScanID := scanID
		cancelMu.Unlock()

		totalPorts := endPort - startPort + 1
		timeout := time.Duration(timeoutMS) * time.Millisecond
		showClosed := showClosedCheck.Checked

		resultsOutput.SetText("")
		statusLabel.SetText(fmt.Sprintf("Scanning %s:%d-%d", host, startPort, endPort))
		summaryLabel.SetText(fmt.Sprintf("Running scan across %d ports.", totalPorts))
		startButton.Disable()
		cancelButton.Enable()
		progress.Show()

		go func() {
			resultsChan := make(chan scanResult)
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup

		portLoop:
			for port := startPort; port <= endPort; port++ {
				select {
				case <-ctx.Done():
					break portLoop
				default:
				}

				wg.Add(1)
				sem <- struct{}{}

				go func(port int) {
					defer wg.Done()
					defer func() { <-sem }()

					select {
					case <-ctx.Done():
						return
					case resultsChan <- scanPort(ctx, host, port, timeout):
					}
				}(port)
			}

			go func() {
				wg.Wait()
				close(resultsChan)
			}()

			var (
				builder        strings.Builder
				openCount      int
				closedCount    int
				processedCount int
			)

			for result := range resultsChan {
				processedCount++
				if result.open {
					openCount++
					builder.WriteString(fmt.Sprintf("OPEN   %d\n", result.port))
				} else {
					closedCount++
					if showClosed {
						builder.WriteString(fmt.Sprintf("CLOSED %d\n", result.port))
					}
				}

				currentOutput := builder.String()
				currentSummary := fmt.Sprintf(
					"Processed %d/%d ports | Open: %d | Closed/Unavailable: %d",
					processedCount,
					totalPorts,
					openCount,
					closedCount,
				)

				fyne.Do(func() {
					resultsOutput.SetText(strings.TrimRight(currentOutput, "\n"))
					summaryLabel.SetText(currentSummary)
				})
			}

			finalStatus := "Scan completed."
			if ctx.Err() != nil {
				finalStatus = "Scan cancelled."
			}

			if builder.Len() == 0 {
				builder.WriteString("No open ports found in the selected range.")
			}

			finalOutput := strings.TrimRight(builder.String(), "\n")
			finalSummary := fmt.Sprintf(
				"%s Processed %d/%d ports | Open: %d | Closed/Unavailable: %d",
				finalStatus,
				processedCount,
				totalPorts,
				openCount,
				closedCount,
			)

			fyne.Do(func() {
				resultsOutput.SetText(finalOutput)
				statusLabel.SetText(finalStatus)
				summaryLabel.SetText(finalSummary)
				progress.Hide()
				startButton.Enable()
				cancelButton.Disable()
			})

			cancelMu.Lock()
			if scanID == currentScanID {
				cancelScan = nil
			}
			cancelMu.Unlock()
		}()
	})
	startButton.Importance = widget.HighImportance

	cancelButton.OnTapped = func() {
		cancelMu.Lock()
		if cancelScan != nil {
			cancelScan()
		}
		cancelMu.Unlock()
		statusLabel.SetText("Cancelling scan...")
		cancelButton.Disable()
	}

	themeButton.OnTapped = func() {
		isDark = !isDark
		setTheme(myApp, isDark, themeButton)
	}

	header := container.NewVBox(
		title,
		subtitle,
	)

	headerCard := widget.NewCard(
		"",
		"",
		container.NewPadded(
			container.NewVBox(
				header,
				widget.NewSeparator(),
				widget.NewLabel("Configure the target, adjust scan behavior, and review results clearly."),
			),
		),
	)

	actionBar := container.NewHBox(
		startButton,
		cancelButton,
		clearButton,
	)

	actionCard := widget.NewCard(
		"Actions",
		"",
		container.NewPadded(actionBar),
	)

	settingsCard := widget.NewCard(
		"Scan Setup",
		"Target and scan behavior",
		container.NewPadded(
			container.NewVBox(
				makeFieldBlock("Host", hostEntry),
				container.NewGridWithColumns(
					2,
					makeFieldBlock("Start Port", startPortEntry),
					makeFieldBlock("End Port", endPortEntry),
				),
				container.NewGridWithColumns(
					2,
					makeFieldBlock("Timeout (ms)", timeoutEntry),
					makeFieldBlock("Concurrency", concurrencyEntry),
				),
				widget.NewSeparator(),
				showClosedCheck,
			),
		),
	)

	statusCard := widget.NewCard(
		"Status",
		"Live scan state",
		container.NewPadded(
			container.NewVBox(
				statusLabel,
				summaryLabel,
				progress,
			),
		),
	)

	resultsHeaderTitle := widget.NewLabelWithStyle("Results", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	resultsHeaderSubtitle := widget.NewLabel("Open ports are always shown. Closed ports are optional.")
	resultsHeaderSubtitle.Wrapping = fyne.TextWrapWord
	themeButtonContainer := container.NewGridWrap(fyne.NewSize(44, 36), themeButton)

	resultsHeader := widget.NewCard(
		"",
		"",
		container.NewPadded(
			container.NewBorder(
				nil,
				nil,
				nil,
				themeButtonContainer,
				container.NewVBox(
					resultsHeaderTitle,
					resultsHeaderSubtitle,
				),
			),
		),
	)

	resultsBodyCard := widget.NewCard(
		"",
		"",
		container.NewPadded(resultsScroll),
	)

	sidebar := container.NewVBox(
		headerCard,
		actionCard,
		settingsCard,
		statusCard,
	)
	sidebarScroll := container.NewVScroll(sidebar)
	sidebarScroll.SetMinSize(fyne.NewSize(320, 0))

	mainArea := container.NewBorder(
		resultsHeader,
		nil,
		nil,
		nil,
		resultsBodyCard,
	)
	split := container.NewHSplit(sidebarScroll, mainArea)
	split.Offset = 0.38

	content := container.NewPadded(split)

	myWindow.SetContent(content)
	myWindow.ShowAndRun()
}
