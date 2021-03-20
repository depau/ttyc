# ttyc

Command-line client for [ttyd](https://github.com/tsl0922/ttyd), [Wi-Se](https://github.com/Depau/wi-se-sw/) and
anything else that uses a compatible protocol.

Features:

- Built-in terminal with a user interface similar to that of [tio](https://github.com/tio/tio)
- Can expose the remote terminal as a pseudo-terminal, which you can connect to with any TTY program (screen, minicom,
  etc.)
- Supports configuring remote UART parameters for Wi-Se

## wistty

Wistty is a utility to set remote terminal parameters for [Wi-Se](https://github.com/Depau/wi-se-sw/).

Wistty is not compatible with ttyd.

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

## Usage

```bash
ttyc --host localhost --port 7681
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

## License

GNU General Public License v3.0 or later
