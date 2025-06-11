.PHONY: build clean install test help

build:
	@echo "Building hello-simple WASM plugin..."
	@go mod tidy
	@tinygo build -o hello.wasm -target wasi main.go
	@echo "Plugin built: hello.wasm"

clean:
	@rm -f hello.wasm
	@echo "Cleaned build artifacts"

install: build
	@mkdir -p ~/.local/share/sup/plugins
	@cp hello.wasm ~/.local/share/sup/plugins/
	@echo "Plugin installed to ~/.local/share/sup/plugins/hello.wasm"

test:
	@if command -v extism >/dev/null 2>&1; then \
		echo "Testing plugin with extism CLI..."; \
		echo "Testing handle_message:"; \
		echo '{"message":"test","sender":"test@example.com","info":{"id":"1","timestamp":1234567890,"push_name":"Test User","is_group":false}}' | extism call hello.wasm handle_message --input-stdin --wasi; \
		echo "\nTesting get_help:"; \
		extism call hello.wasm get_help --wasi; \
	else \
		echo "extism CLI not found. Install it to test the plugin."; \
		echo "Visit: https://extism.org/docs/install"; \
	fi

help:
	@echo "Available targets:"
	@echo "  build   - Build the WASM plugin"
	@echo "  clean   - Remove build artifacts"
	@echo "  install - Build and install plugin to ~/.local/share/sup/plugins"
	@echo "  test    - Test the plugin with extism CLI (requires extism CLI)"
	@echo "  help    - Show this help message"