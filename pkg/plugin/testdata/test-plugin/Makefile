.PHONY: build clean

build: test-plugin.wasm

test-plugin.wasm: main.go go.mod
	tinygo build -o test-plugin.wasm -target wasi main.go

clean:
	rm -f test-plugin.wasm

deps:
	go mod tidy
