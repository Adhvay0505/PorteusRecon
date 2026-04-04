package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"image/color"
	"io"
	"net"
	"os/exec"
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
	port    int
	open    bool
	service string
	err     error
}

type scanProfile struct {
	Name        string
	Description string
	Args        []string
	PostArgs    []string
}

type builtInScanOptions struct {
	skipHostDiscovery bool
	serviceDetection  bool
	osDetection       bool
	aggressive        bool
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

func probeService(host string, port int, conn net.Conn, timeout time.Duration) string {
	_ = conn.SetDeadline(time.Now().Add(timeout))

	buffer := make([]byte, 512)
	trimBanner := func(n int) string {
		return strings.TrimSpace(strings.ReplaceAll(string(buffer[:n]), "\x00", ""))
	}

	switch port {
	case 22:
		if n, err := conn.Read(buffer); err == nil && n > 0 {
			return "ssh: " + trimBanner(n)
		}
	case 21, 25, 110, 143, 587:
		if n, err := conn.Read(buffer); err == nil && n > 0 {
			return trimBanner(n)
		}
	case 80, 8080, 8000, 8888:
		_, _ = io.WriteString(conn, fmt.Sprintf("HEAD / HTTP/1.0\r\nHost: %s\r\n\r\n", host))
		if n, err := conn.Read(buffer); err == nil && n > 0 {
			firstLine := strings.Split(trimBanner(n), "\n")[0]
			return strings.TrimSpace(firstLine)
		}
	case 443, 8443:
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: true,
		})
		defer tlsConn.Close()
		if err := tlsConn.Handshake(); err == nil {
			state := tlsConn.ConnectionState()
			if state.NegotiatedProtocol != "" {
				return "tls: " + state.NegotiatedProtocol
			}
			return "tls enabled"
		}
		return "tls service detected"
	}

	if n, err := conn.Read(buffer); err == nil && n > 0 {
		return trimBanner(n)
	}

	return ""
}

func scanPort(ctx context.Context, host string, port int, timeout time.Duration, detectService bool) scanResult {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: timeout}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return scanResult{port: port, err: err}
	}

	result := scanResult{port: port, open: true}
	if detectService {
		result.service = probeService(host, port, conn, timeout)
	}
	_ = conn.Close()

	return result
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

func profilesForEngine(engine string) []scanProfile {
	switch engine {
	case "Nmap":
		return []scanProfile{
			{Name: "Default TCP Connect", Description: "Runs a straightforward TCP connect scan across the selected port range.", Args: []string{"-sT"}},
			{Name: "Service Detection (-sV)", Description: "Attempts to identify the service and version running on open ports.", Args: []string{"-sV"}},
			{Name: "OS Detection (-O)", Description: "Enables operating system detection for hosts that expose enough fingerprint data.", Args: []string{"-O"}},
			{Name: "Aggressive Scan (-A)", Description: "Enables version detection, OS detection, default scripts, and traceroute.", Args: []string{"-A"}},
			{Name: "Skip Host Discovery (-Pn)", Description: "Treats the host as online and skips the default discovery phase.", Args: []string{"-Pn"}},
		}
	case "RustScan":
		return []scanProfile{
			{Name: "Default RustScan", Description: "Runs RustScan with its standard behavior over the selected port range."},
			{Name: "Higher Ulimit (--ulimit 5000)", Description: "Raises the file-descriptor ceiling for faster larger scans when the system allows it.", Args: []string{"--ulimit", "5000"}},
			{Name: "Longer Timeout (--timeout 4000)", Description: "Waits longer for slow services before marking ports as closed.", Args: []string{"--timeout", "4000"}},
			{Name: "Serial Scan Order", Description: "Scans ports in ascending order instead of using a randomized order.", Args: []string{"--scan-order", "Serial"}},
			{Name: "Random Scan Order", Description: "Randomizes the port order to vary the scan pattern.", Args: []string{"--scan-order", "Random"}},
			{Name: "RustScan + Nmap Aggressive", Description: "Uses RustScan to find ports quickly, then hands the results to Nmap with the aggressive -A profile.", PostArgs: []string{"-A"}},
		}
	default:
		return []scanProfile{
			{Name: "Built-in TCP Scan", Description: "Uses the internal Go TCP connect scanner across the selected port range."},
		}
	}
}

func profileNames(profiles []scanProfile) []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	return names
}

func selectedProfileByName(profiles []scanProfile, name string) scanProfile {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile
		}
	}
	if len(profiles) == 0 {
		return scanProfile{}
	}
	return profiles[0]
}

func buildCommandPreview(engine string, profile scanProfile, host string, startPort int, endPort int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "<host>"
	}

	portRange := fmt.Sprintf("%d-%d", startPort, endPort)

	switch engine {
	case "Nmap":
		args := append([]string{}, profile.Args...)
		args = append(args, "-p", portRange, host)
		return "nmap " + strings.Join(args, " ")
	case "RustScan":
		args := []string{"-a", host, "--range", portRange}
		args = append(args, profile.Args...)
		if len(profile.PostArgs) > 0 {
			args = append(args, "--")
			args = append(args, profile.PostArgs...)
		}
		return "rustscan " + strings.Join(args, " ")
	default:
		return fmt.Sprintf("Built-in TCP scan against %s on ports %s", host, portRange)
	}
}

func runExternalScan(ctx context.Context, binary string, args []string) (string, error) {
	if _, err := exec.LookPath(binary); err != nil {
		return "", fmt.Errorf("%s is not installed or not in PATH", binary)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func hasBinary(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

func builtInOptionsFor(engine string, profile scanProfile) builtInScanOptions {
	options := builtInScanOptions{}
	if engine != "Nmap" {
		return options
	}

	switch profile.Name {
	case "Service Detection (-sV)":
		options.serviceDetection = true
	case "OS Detection (-O)":
		options.osDetection = true
	case "Aggressive Scan (-A)":
		options.serviceDetection = true
		options.osDetection = true
		options.aggressive = true
	case "Skip Host Discovery (-Pn)":
		options.skipHostDiscovery = true
	}

	return options
}

func discoverHost(host string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("host is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		return nil
	}
	_, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("host discovery failed: %w", err)
	}
	return nil
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

	scanEngineSelect := widget.NewSelect([]string{"Built-in TCP", "Nmap", "RustScan"}, nil)
	scanEngineSelect.SetSelected("Built-in TCP")

	activeProfiles := profilesForEngine(scanEngineSelect.Selected)
	scanProfileSelect := widget.NewSelect(profileNames(activeProfiles), nil)
	scanProfileSelect.SetSelected(activeProfiles[0].Name)

	profileDescriptionLabel := widget.NewLabel(activeProfiles[0].Description)
	profileDescriptionLabel.Wrapping = fyne.TextWrapWord

	commandPreviewLabel := widget.NewLabel(buildCommandPreview(scanEngineSelect.Selected, activeProfiles[0], hostEntry.Text, 1, 1024))
	commandPreviewLabel.Wrapping = fyne.TextWrapWord

	showClosedCheck := widget.NewCheck("Show closed ports in results", nil)

	statusLabel := widget.NewLabel("Ready")
	statusLabel.Wrapping = fyne.TextWrapWord
	summaryLabel := widget.NewLabel("No scan has been run yet.")
	summaryLabel.Wrapping = fyne.TextWrapWord

	resultsOutput := widget.NewLabel("Open ports and scan messages will appear here.")
	resultsOutput.Wrapping = fyne.TextWrapWord

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

	updateScanProfileUI := func() {
		activeProfiles = profilesForEngine(scanEngineSelect.Selected)
		scanProfileSelect.Options = profileNames(activeProfiles)
		scanProfileSelect.SetSelected(activeProfiles[0].Name)
		profileDescriptionLabel.SetText(activeProfiles[0].Description)

		startPort, _ := strconv.Atoi(startPortEntry.Text)
		endPort, _ := strconv.Atoi(endPortEntry.Text)
		if startPort <= 0 {
			startPort = 1
		}
		if endPort <= 0 {
			endPort = 1024
		}
		commandPreviewLabel.SetText(buildCommandPreview(scanEngineSelect.Selected, activeProfiles[0], hostEntry.Text, startPort, endPort))
	}

	scanEngineSelect.OnChanged = func(string) {
		updateScanProfileUI()
	}

	scanProfileSelect.OnChanged = func(selected string) {
		profile := selectedProfileByName(activeProfiles, selected)
		profileDescriptionLabel.SetText(profile.Description)

		startPort, _ := strconv.Atoi(startPortEntry.Text)
		endPort, _ := strconv.Atoi(endPortEntry.Text)
		if startPort <= 0 {
			startPort = 1
		}
		if endPort <= 0 {
			endPort = 1024
		}
		commandPreviewLabel.SetText(buildCommandPreview(scanEngineSelect.Selected, profile, hostEntry.Text, startPort, endPort))
	}

	hostEntry.OnChanged = func(string) {
		profile := selectedProfileByName(activeProfiles, scanProfileSelect.Selected)
		startPort, _ := strconv.Atoi(startPortEntry.Text)
		endPort, _ := strconv.Atoi(endPortEntry.Text)
		if startPort <= 0 {
			startPort = 1
		}
		if endPort <= 0 {
			endPort = 1024
		}
		commandPreviewLabel.SetText(buildCommandPreview(scanEngineSelect.Selected, profile, hostEntry.Text, startPort, endPort))
	}

	startPortEntry.OnChanged = func(string) {
		profile := selectedProfileByName(activeProfiles, scanProfileSelect.Selected)
		startPort, _ := strconv.Atoi(startPortEntry.Text)
		endPort, _ := strconv.Atoi(endPortEntry.Text)
		if startPort <= 0 {
			startPort = 1
		}
		if endPort <= 0 {
			endPort = 1024
		}
		commandPreviewLabel.SetText(buildCommandPreview(scanEngineSelect.Selected, profile, hostEntry.Text, startPort, endPort))
	}

	endPortEntry.OnChanged = func(string) {
		profile := selectedProfileByName(activeProfiles, scanProfileSelect.Selected)
		startPort, _ := strconv.Atoi(startPortEntry.Text)
		endPort, _ := strconv.Atoi(endPortEntry.Text)
		if startPort <= 0 {
			startPort = 1
		}
		if endPort <= 0 {
			endPort = 1024
		}
		commandPreviewLabel.SetText(buildCommandPreview(scanEngineSelect.Selected, profile, hostEntry.Text, startPort, endPort))
	}

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
		selectedEngine := scanEngineSelect.Selected
		selectedProfile := selectedProfileByName(activeProfiles, scanProfileSelect.Selected)

		resultsOutput.SetText("")
		statusLabel.SetText(fmt.Sprintf("%s scanning %s:%d-%d", selectedEngine, host, startPort, endPort))
		summaryLabel.SetText(fmt.Sprintf("Running scan across %d ports.", totalPorts))
		startButton.Disable()
		cancelButton.Enable()
		progress.Show()

		go func() {
			useExternal := selectedEngine == "RustScan" || (selectedEngine == "Nmap" && hasBinary("nmap"))
			if useExternal {
				var (
					binary string
					args   []string
				)

				portRange := fmt.Sprintf("%d-%d", startPort, endPort)
				switch selectedEngine {
				case "Nmap":
					binary = "nmap"
					args = append([]string{}, selectedProfile.Args...)
					args = append(args, "-p", portRange, host)
				case "RustScan":
					binary = "rustscan"
					args = []string{"-a", host, "--range", portRange}
					args = append(args, selectedProfile.Args...)
					if len(selectedProfile.PostArgs) > 0 {
						args = append(args, "--")
						args = append(args, selectedProfile.PostArgs...)
					}
				}

				output, err := runExternalScan(ctx, binary, args)
				finalStatus := fmt.Sprintf("%s scan completed.", selectedEngine)
				if ctx.Err() != nil {
					finalStatus = fmt.Sprintf("%s scan cancelled.", selectedEngine)
				} else if err != nil {
					finalStatus = fmt.Sprintf("%s scan failed.", selectedEngine)
				}

				if output == "" && err != nil {
					output = err.Error()
				}
				if output == "" {
					output = "No output returned."
				}
				finalSummary := fmt.Sprintf("%s Profile: %s", finalStatus, selectedProfile.Name)

				fyne.Do(func() {
					resultsOutput.SetText(output)
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
				return
			}

			builtInOptions := builtInOptionsFor(selectedEngine, selectedProfile)
			if !builtInOptions.skipHostDiscovery {
				if err := discoverHost(host); err != nil {
					fyne.Do(func() {
						resultsOutput.SetText(err.Error())
						statusLabel.SetText("Scan failed.")
						summaryLabel.SetText(fmt.Sprintf("Profile: %s", selectedProfile.Name))
						progress.Hide()
						startButton.Enable()
						cancelButton.Disable()
					})

					cancelMu.Lock()
					if scanID == currentScanID {
						cancelScan = nil
					}
					cancelMu.Unlock()
					return
				}
			}

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
					case resultsChan <- scanPort(ctx, host, port, timeout, builtInOptions.serviceDetection || builtInOptions.aggressive):
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
					if result.service != "" {
						builder.WriteString(fmt.Sprintf("OPEN   %d   %s\n", result.port, result.service))
					} else {
						builder.WriteString(fmt.Sprintf("OPEN   %d\n", result.port))
					}
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
			if selectedEngine == "Nmap" && !hasBinary("nmap") {
				if builtInOptions.osDetection || builtInOptions.aggressive {
					finalSummary += " | OS fingerprint data limited in current scan mode"
				} else {
					finalSummary += " | Profile behavior applied"
				}
			}

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
					makeFieldBlock("Scan Engine", scanEngineSelect),
					makeFieldBlock("Flag Profile", scanProfileSelect),
				),
				makeFieldBlock("Profile Description", profileDescriptionLabel),
				makeFieldBlock("Command Preview", commandPreviewLabel),
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
