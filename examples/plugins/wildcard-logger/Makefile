.PHONY: build clean

PLUGIN_NAME = wildcard-logger
WASM_FILE = $(PLUGIN_NAME).wasm

build:
	@echo "Building $(PLUGIN_NAME) plugin..."
	tinygo build -o $(WASM_FILE) -target wasi main.go
	@echo "Plugin built: $(WASM_FILE)"

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(WASM_FILE)

install: build
	@echo "Installing plugin to ~/.local/share/sup/plugins/..."
	@mkdir -p ~/.local/share/sup/plugins
	cp $(WASM_FILE) ~/.local/share/sup/plugins/$(WASM_FILE)
	@echo "Plugin installed successfully"

uninstall:
	@echo "Uninstalling plugin..."
	rm -f ~/.local/share/sup/plugins/$(WASM_FILE)
	@echo "Plugin uninstalled"