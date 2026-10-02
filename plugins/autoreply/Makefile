.PHONY: build clean install

build:
	tinygo build -o autoreply.wasm -target wasi .

clean:
	rm -f autoreply.wasm

install: build
	mkdir -p $(HOME)/.local/share/sup/plugins
	cp autoreply.wasm $(HOME)/.local/share/sup/plugins/
