module github.com/Depau/ttyc

go 1.15

require (
	github.com/Depau/switzerland v0.0.0-20210627232915-baff755487c4
	github.com/TwinProduction/go-color v1.0.0
	github.com/containerd/console v1.0.2
	github.com/lestrrat-go/strftime v1.0.4
	github.com/mattn/go-isatty v0.0.12
	github.com/mkideal/cli v0.2.5
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	nhooyr.io/websocket v1.8.6
)

// Fork that does not enable OPOST on raw TTY
replace github.com/containerd/console v1.0.2 => github.com/Depau/console v1.0.2-0.20210627210027-beeaa4cc766b
