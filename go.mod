module github.com/benaskins/lamina

go 1.26.0

require (
	github.com/benaskins/axon-eval v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.10.2
	golang.org/x/mod v0.33.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/benaskins/axon-eval => ./axon-eval

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)
