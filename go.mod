module gb-cms

go 1.19

require (
	github.com/ghettovoice/gosip v0.0.0-20240401112151-56d750b16008
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.1
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/sirupsen/logrus v1.9.3
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.21.0
	golang.org/x/text v0.16.0
)

require (
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/discoviking/fsm v0.0.0-20150126104936-f4a273feecca // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/satori/go.uuid v1.2.1-0.20181028125025-b2ce2384e17b // indirect
	github.com/tevino/abool v1.2.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.24.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/term v0.21.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

require github.com/lkmio/avformat v0.0.0

replace github.com/lkmio/avformat => ../avformat
