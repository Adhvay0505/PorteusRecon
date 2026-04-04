# PorteusRecon

A simple port scanner built in Go with both CLI and Fyne GUI entrypoints.

## Technologies Used

- Go
- Fyne GUI toolkit

## Run From Source

Clone the repository:

```bash
git clone https://github.com/Adhvay0505/PorteusRecon.git
cd PorteusRecon
```

Run the CLI:

```bash
go run PorteusReconCLI.go -host 127.0.0.1 -start 1 -end 1024
```

Run the GUI:

```bash
go run PorteusReconGUI.go
```

## Build Binaries

Create the external build directory:

```bash
mkdir -p "$HOME/PorteusRecon-dist"
```

Build the CLI outside the repository:

```bash
go build -o "$HOME/PorteusRecon-dist/PorteusReconCLI" PorteusReconCLI.go
```

Build the GUI outside the repository:

```bash
go build -o "$HOME/PorteusRecon-dist/PorteusReconGUI" PorteusReconGUI.go
```

Run the built binaries:

```bash
"$HOME/PorteusRecon-dist/PorteusReconCLI" -host 127.0.0.1 -start 1 -end 1024
"$HOME/PorteusRecon-dist/PorteusReconGUI"
```

The repository no longer uses an in-repo `dist/` directory for build artifacts.

## GUI Scan Profiles

The GUI now includes:

- Built-in TCP scan mode
- Nmap-style scan profiles with command previews
- RustScan profile selection with descriptions

When the required external scanner is available, the GUI can use it automatically. Release artifacts should still be built to a directory under the user home directory, such as `$HOME/PorteusRecon-dist`.

## Linux GUI Dependencies

On Arch Linux, the Fyne desktop build needs system graphics libraries:

```bash
sudo pacman -Sy --needed base-devel pkgconf mesa libx11 libxcursor libxinerama libxrandr libxi
```

## Packaged GUI Artifact

Build release artifacts outside the repository:

```bash
mkdir -p "$HOME/PorteusRecon-dist"
go build -o "$HOME/PorteusRecon-dist/PorteusReconGUI" PorteusReconGUI.go
tar -czf "$HOME/PorteusRecon-dist/PorteusReconGUI_linux_amd64.tar.gz" -C "$HOME/PorteusRecon-dist" PorteusReconGUI
```

By default this writes to:

```bash
$HOME/PorteusRecon-dist/PorteusReconGUI
$HOME/PorteusRecon-dist/PorteusReconGUI_linux_amd64.tar.gz
```


## Dark Mode
### New 
<img width="1885" height="945" alt="Screenshot_2026-04-03_19-33-34" src="https://github.com/user-attachments/assets/08df9985-dd7c-4701-a5de-d2cfb47d0e71" />

<img width="1030" height="744" alt="Screenshot From 2025-07-29 19-45-28" src="https://github.com/user-attachments/assets/213ca53a-7ae8-4944-accc-a735c767dd5b" />

## Light Mode
### New
<img width="1885" height="945" alt="Screenshot_2026-04-03_19-33-42" src="https://github.com/user-attachments/assets/bb8cddd7-107f-4d4d-9461-1cc4a0f04d27" />

<img width="1030" height="744" alt="Screenshot From 2025-07-29 19-46-09" src="https://github.com/user-attachments/assets/5025b914-9eca-4a63-80cd-936aef843581" />
