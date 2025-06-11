.PHONY: build clean install test help setup deps

build:
	@echo "Building hello-javascript WASM plugin..."
	@mkdir -p dist
	@if ! command -v extism-js >/dev/null 2>&1; then \
		echo "Error: extism-js not found. Install it with:"; \
		echo "  curl -O https://raw.githubusercontent.com/extism/js-pdk/main/install.sh"; \
		echo "  sh install.sh"; \
		exit 1; \
	fi
	@extism-js index.js -i plugin.d.ts -o dist/plugin.wasm
	@echo "Plugin built: dist/plugin.wasm"

clean:
	@rm -rf dist/
	@echo "Cleaned build artifacts"

install: build
	@mkdir -p ~/.local/share/sup/plugins
	@cp dist/plugin.wasm ~/.local/share/sup/plugins/hello-js.wasm
	@echo "Plugin installed to ~/.local/share/sup/plugins/hello-js.wasm"

test:
	@if command -v extism >/dev/null 2>&1; then \
		echo "Testing plugin with extism CLI..."; \
		echo "Testing handle_message:"; \
		echo '{"message":"test","sender":"test@example.com","info":{"id":"1","timestamp":1234567890,"push_name":"Test User","is_group":false}}' | extism call dist/plugin.wasm handle_message --input-stdin --wasi; \
		echo "\nTesting get_help:"; \
		extism call dist/plugin.wasm get_help --wasi; \
	else \
		echo "extism CLI not found. Install it to test the plugin."; \
		echo "Visit: https://extism.org/docs/install"; \
	fi

deps:
	@echo "Installing JavaScript dependencies..."
	@if command -v npm >/dev/null 2>&1; then \
		npm install; \
	else \
		echo "npm not found. Please install Node.js and npm."; \
		exit 1; \
	fi

setup: deps
	@echo "Setting up development environment..."
	@if ! command -v extism-js >/dev/null 2>&1; then \
		echo "Installing extism-js compiler..."; \
		curl -O https://raw.githubusercontent.com/extism/js-pdk/main/install.sh; \
		sh install.sh; \
		rm install.sh; \
	fi
	@echo "Setup complete!"

help:
	@echo "Available targets:"
	@echo "  build   - Build the WASM plugin from JavaScript source"
	@echo "  clean   - Remove build artifacts"
	@echo "  install - Build and install plugin to ~/.local/share/sup/plugins"
	@echo "  test    - Test the plugin with extism CLI (requires extism CLI)"
	@echo "  deps    - Install JavaScript dependencies"
	@echo "  setup   - Install dependencies and build tools"
	@echo "  help    - Show this help message"
	@echo
	@echo "Prerequisites:"
	@echo "  - Node.js 16+"
	@echo "  - npm"
	@echo "  - extism-js compiler (installed via 'make setup')"
	@echo "  - extism CLI (optional, for testing)"