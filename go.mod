module tts

go 1.25

require (
	github.com/nnikolov3/configurator v0.0.0
	golang.org/x/sync v0.16.0
	logger v0.0.0
)

require github.com/pelletier/go-toml/v2 v2.2.4 // indirect

replace github.com/nnikolov3/configurator => ../configurator

replace logger => ../logger
