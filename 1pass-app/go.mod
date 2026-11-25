// Dependencies for 1Pass application.
//
// @author TSS

module github.com/mashmb/1pass/1pass-app

go 1.21

require (
	github.com/jedib0t/go-pretty/v6 v6.2.1
	github.com/jroimartin/gocui v0.4.0
	github.com/mashmb/1pass/1pass-core v1.3.1
	github.com/mashmb/1pass/1pass-parse v1.3.1
	github.com/mashmb/1pass/1pass-term v1.3.1
	github.com/mashmb/1pass/1pass-up v1.1.2
	github.com/spf13/cobra v1.1.3
	golang.org/x/crypto v0.21.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/nsf/termbox-go v1.1.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/term v0.18.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/sqlite v1.34.5 // indirect
)

replace (
	github.com/mashmb/1pass/1pass-core => ../1pass-core
	github.com/mashmb/1pass/1pass-parse => ../1pass-parse
	github.com/mashmb/1pass/1pass-term => ../1pass-term
	github.com/mashmb/1pass/1pass-up => ../1pass-up
)
