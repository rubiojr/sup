.PHONY: build clean install test

PLUGIN_NAME = eat-bcn
WASM_FILE = $(PLUGIN_NAME).wasm
INSTALL_DIR = $(HOME)/.local/share/sup/plugins

build:
	tinygo build -o $(WASM_FILE) -target wasi main.go

clean:
	rm -f $(WASM_FILE)

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(WASM_FILE) $(INSTALL_DIR)/

test: build
	@echo "Testing plugin with extism..."
	@echo '{"message":"","sender":"test@example.com","info":{"id":"test","timestamp":1234567890,"push_name":"Test User","is_group":false}}' | extism call $(WASM_FILE) handle_message --input=-

help:
	@echo "Available targets:"
	@echo "  build   - Build the WASM plugin"
	@echo "  clean   - Remove built files"
	@echo "  install - Build and install to plugin directory"
	@echo "  test    - Test the plugin with sample input"
	@echo "  help    - Show this help message"

all: build
