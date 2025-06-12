module eat-bcn

go 1.24.3

toolchain go1.24.4

require github.com/rubiojr/sup/pkg/plugin v0.0.0

require (
	github.com/extism/go-pdk v1.1.3 // indirect
	github.com/rubiojr/sup v0.2.1 // indirect
)

replace github.com/rubiojr/sup/pkg/plugin => ../../pkg/plugin
