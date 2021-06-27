module github.com/Depau/ttyc

go 1.15

require (
	github.com/containerd/console v1.0.2
	github.com/lestrrat-go/strftime v1.0.4
	github.com/mattn/go-isatty v0.0.12
	github.com/mkideal/cli v0.2.5
	github.com/tHinqa/outside v0.0.0-20131227223926-48a9c99b2195 // indirect
	github.com/tHinqa/outside-windows v0.0.0-20131225231147-79e174abeec9
	nhooyr.io/websocket v1.8.6
)

// Fork that does not enable OPOST on raw TTY
replace github.com/containerd/console v1.0.2 => github.com/Depau/console v1.0.2-0.20210627210027-beeaa4cc766b
