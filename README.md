# ttyc

Command-line client for [ttyd](https://github.com/tsl0922/ttyd), [Wi-Se](https://github.com/Depau/wi-se-sw/) and
anything else that uses a compatible protocol.

Features:

- Works on all major operating systems including Windows
- Built-in terminal with a user interface similar to that of [tio](https://github.com/tio/tio)
- Supports configuring remote UART parameters for Wi-Se

Additionally, on all platforms except Windows and macOS:

- Can expose the remote terminal as a pseudo-terminal, which you can connect to with any TTY program (screen, minicom,
  etc.)

## wistty

Wistty is a utility to set remote terminal parameters for [Wi-Se](https://github.com/Depau/wi-se-sw/).

Wistty is not compatible with ttyd.

## Installation

### Releases

Head over to the [Releases](https://github.com/Depau/ttyc/releases) page and grab a prebuilt binary.

### Arch Linux

Packages are available in the AUR:

- `ttyc` - https://aur.archlinux.org/packages/ttyc/
- `wistty` - https://aur.archlinux.org/packages/wistty/
- `ttyc-git` - https://aur.archlinux.org/packages/ttyc-git/ (provides `wistty`)

## Building

### Quick way

```bash
go get github.com/Depau/ttyc/cmd/ttyc
go get github.com/Depau/ttyc/cmd/wistty
```

Binaries will be saved to `$GOHOME/bin/ttyc` and `$GOHOME/bin/wistty`, usually `~/go/bin/...`

### From local sources

```bash
git clone https://github.com/Depau/ttyc.git
cd ttyc/cmd/ttyc
go build
cd ../wistty
go build
```

### Build locally for all platforms

```bash
git clone https://github.com/Depau/ttyc.git
cd ttyc
go run .ci/ci_build.go
```

Outputs will be placed in the `build/` directory.

## Usage

```bash
ttyc --url http://localhost:7681
```

```
  -h, --help                Show help
  -U, --url                 Server URL
  -w, --watchdog[=2]        WebSocket ping interval in seconds, 0 to disable, default 2.
  -r, --reconnect[=2]       Reconnection interval in seconds, -1 to disable, default 3.
      --backoff[=none]      Backoff type, none, linear, exponential, defaults to linear
      --backoff-value[=2]   For linear backoff, increase reconnect interval by this amount of seconds after each iteration. For exponential backoff, multiply reconnect interval by this amount. Default 2
  -u, --user                Username for authentication
  -k, --pass                Password for authentication
  -T, --tty                 Do not launch terminal, create terminal device at given location (i.e. /tmp/ttyd)
  -b, --baudrate[=-1]       (Wi-Se only) Set remote baud rate [bps]
  -p, --parity              (Wi-Se only) Set remote parity [odd|even|none]
  -d, --databits[=-1]       (Wi-Se only) Set remote data bits [5|6|7|8]
  -s, --stopbits[=-1]       (Wi-Se only) Set remote stop bits [1|2]
  -v, --version             Show version
```

```bash
wistty (ttyc) - Manage Wi-Se remote terminal parameters

Options:

  -h, --help            Show help
  -U, --url             Server URL
  -u, --user            Username for authentication
  -k, --pass            Password for authentication
  -j, --json            Return machine-readable JSON output
  -b, --baudrate[=-1]   Set remote baud rate [bps]
  -p, --parity          Set remote parity [odd|even|none]
  -d, --databits[=-1]   Set remote data bits [5|6|7|8]
  -s, --stopbits[=-1]   Set remote stop bits [1|2]
  -v, --version         Show version
```

## Multiplatform notes

### GNU/Linux

All functionality is available. This software is mainly tested on GNU/Linux so it should work well.

### Windows

Occasionally tested.

Raw terminal has been ported and workarounds have been implemented for missing system functionality.
Everything should work perfectly except for TTY support, which is not supported by Windows itself (afaik).

Text selection is disabled since it messes up the WebSocket communication. If you know workarounds, hit me up.

### macOS

Occasionally tested, everything except for TTY support should work well.,

### Android (with Bionic libc)

It only seems to build for aarch64. Not tested. Other builds may work with with additional configuration tweaks.
TTY support is likely to require additional permissions.

### OpenWRT (or other distros with uClibc)

Tested on recent releases, it should work very well but huge 5MB binaries don't make it a very good fit on cheap routers.

### Other Non-GNU/Linux (i.e musl libc)

Not tested, will likely work but YMMV.

### Other BSD

Not tested, most things will likely work. TTY support may work or it may crash the program.

## License

GNU General Public License v3.0 or later
