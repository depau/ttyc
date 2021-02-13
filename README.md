# ttyc

Command-line client for [ttyd](https://github.com/tsl0922/ttyd), [Wi-Se](https://github.com/Depau/wi-se-sw/) and
anything else that uses a compatible protocol.

Features:

- Built-in terminal with a user interface compatible to that of [tio](https://github.com/tio/tio)
- Can expose the remote terminal as a pseudo-terminal, which you can connect to with any TTY program (screen, minicom,
  etc.)
- Supports configuring remote UART parameters for Wi-Se

## Building

### Quick way

```bash
go get github.com/Depau/ttyc/cmd/ttyc
```

Binary will be saved to `$GOHOME/bin/ttyc`, usually `~/go/bin/ttyc`

### From local sources

```bash
git clone https://github.com/Depau/ttyc.git
cd ttyc/cmd/ttyc
go build
```

## Usage

```bash
ttyc --host localhost --port 7681
```

```
  -h, --help                Show help
  -H, --host               *Server hostname
  -P, --port               *Server port
  -t, --tls[=false]         Use TLS
  -w, --watchdog[=10]       WebSocket ping interval in seconds, 0 to disable, default 10.
  -r, --reconnect[=3]       Reconnection interval in seconds, -1 to disable, default 3.
      --backoff[=linear]    Backoff type, none, linear, exponential, defaults to linear
      --backoff-value[=2]   For linear backoff, increase reconnect interval by this amount of seconds
                            after each iteration. For exponential backoff, multiply reconnect interval
                            by this amount. Default 2,
  -u, --user                Username for authentication
  -k, --pass                Password for authentication
  -T, --tty                 Do not launch terminal, create terminal device at given location (i.e. /tmp/ttyd)
  -b, --baudrate[=-1]       (Wi-Se only) Set remote baud rate [bps]
  -p, --parity              (Wi-Se only) Set remote parity [odd|even|none]
  -d, --databits[=-1]       (Wi-Se only) Set remote data bits [5|6|7|8]
  -s, --stopbits[=-1]       (Wi-Se only) Set remote stop bits [1|2]
```

## License

GNU General Public License v3.0 or later