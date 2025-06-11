module hello-simple

go 1.21.0

toolchain go1.24.3

require github.com/rubiojr/sup/pkg/plugin v0.0.0-00010101000000-000000000000

require github.com/extism/go-pdk v1.0.2 // indirect

replace github.com/rubiojr/sup/pkg/plugin => ../../../pkg/plugin
