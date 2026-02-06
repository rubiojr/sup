.PHONY: build clean install test help

build:
	@echo "Building agendalo WASM plugin..."
	@go mod tidy
	@tinygo build -o agendalo.wasm -target wasi main.go
	@echo "Plugin built: agendalo.wasm"

clean:
	@rm -f agendalo.wasm
	@echo "Cleaned build artifacts"

install: build
	@mkdir -p ~/.local/share/sup/plugins
	@cp agendalo.wasm ~/.local/share/sup/plugins/
	@echo "Plugin installed to ~/.local/share/sup/plugins/agendalo.wasm"

test:
	@if command -v extism >/dev/null 2>&1; then \
		echo "Testing plugin with extism CLI..."; \
		echo "Testing handle_message:"; \
		echo '{"message":"list","sender":"test@example.com","info":{"id":"1","timestamp":1234567890,"push_name":"Test User","is_group":false}}' | extism call agendalo.wasm handle_message --input-stdin --wasi; \
		echo "\nTesting get_help:"; \
		extism call agendalo.wasm get_help --wasi; \
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
