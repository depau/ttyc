module github.com/Depau/ttyc

go 1.15

require (
	github.com/containerd/console v1.0.1
	github.com/gorilla/websocket v1.4.2
	github.com/kr/pretty v0.2.1 // indirect
	github.com/lestrrat-go/strftime v1.0.4
	github.com/mattn/go-isatty v0.0.12
	github.com/mkideal/cli v0.2.3
)

// Fork that does not enable OPOST on raw TTY
replace github.com/containerd/console v1.0.1 => github.com/Depau/console v1.0.2-0.20210314195305-ff8df53f5172
