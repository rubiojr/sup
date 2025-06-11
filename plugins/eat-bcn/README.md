# Barcelona Restaurants Plugin

A WASM plugin for Sup that provides random Barcelona restaurant suggestions.

## Overview

This plugin converts the built-in Barcelona restaurant handler to an external WASM plugin. It provides 3 random restaurant suggestions from a curated list of Barcelona restaurants, complete with ratings, cost indicators, cuisine types, and website links.

## Features

- 🍽️ Random selection of 3 restaurants from 20+ options
- ⭐ Star ratings (1-3 stars)
- 💰 Cost indicators ($, $$, $$$)
- 🌍 Cuisine type information
- 🔗 Direct links to restaurant websites
- 🍻 Fun Spanish farewell message

## Usage

The plugin responds to several command aliases:

```
.sup micheladas
.sup cenar
.sup bcn-eat
```

All commands produce the same result: 3 random restaurant suggestions.

## Sample Output

```
🍻 Here are 3 random Barcelona restaurant suggestions:

🍽️ Cal Pep
🔗 https://calpep.com
Cost: $$
Rating: ⭐⭐⭐
Cuisine: Tapas

🥘 Disfrutar
🔗 https://disfrutarbarcelona.com
Cost: $$$
Rating: ⭐⭐⭐
Cuisine: Modern Catalan

🍴 Quimet & Quimet
🔗 https://quimetquimet.com
Cost: $
Rating: ⭐⭐⭐
Cuisine: Tapas

¡Buen provecho! 🎉
```

## Building

### Prerequisites

- [TinyGo](https://tinygo.org/getting-started/install/) installed
- Go 1.21 or later

### Build Commands

```bash
# Build the plugin
make build

# Build and install to plugin directory
make install

# Clean build artifacts
make clean

# Test the plugin (requires extism CLI)
make test
```

## Installation

1. Build the plugin:
   ```bash
   make build
   ```

2. Install to the plugin directory:
   ```bash
   make install
   ```

3. Restart Sup or reload plugins:
   ```bash
   sup plugins reload
   ```

## Restaurant Data

The plugin currently uses embedded restaurant data for simplicity. The restaurants include:

- **Traditional Catalan**: Can Culleretes, Pinotxo Bar, Cal Boter
- **Modern Catalan**: Disfrutar
- **Tapas**: Cal Pep, Bar Mut, Quimet & Quimet, La Pepita, Cervecería Catalana, Bar del Pla, El Xampanyet, Bodega 1900, Paco Meralgo, La Cova Fumada
- **Creative Tapas**: Tickets
- **Basque**: Sagardi
- **Wine Bar**: La Vinya del Senyor
- **Belgian Tapas**: Gilda by Belgious
- **Patatas Bravas**: Bar Tomás

### Customizing Restaurant Data

To add or modify restaurants, edit the `restaurantData` variable in `main.go`. The format is:

```
Name#URL#Cuisine#Cost#Rating
```

Where:
- **Name**: Restaurant name (with optional number prefix)
- **URL**: Website URL
- **Cuisine**: Type of cuisine
- **Cost**: 1-3 ($ to $$$)
- **Rating**: 1-3 (⭐ to ⭐⭐⭐)

## Differences from Built-in Handler

This plugin version differs from the original built-in handler in the following ways:

1. **Data Storage**: Uses embedded data instead of reading from external files
2. **Error Handling**: Uses plugin-specific error handling patterns
3. **Interface**: Uses the simplified Plugin interface instead of direct WhatsApp client access
4. **Deployment**: Deployed as a WASM plugin instead of compiled into the main binary

## Development Notes

- The plugin uses the Sup Plugin Development Kit (PDK) for simplified development
- All restaurant data is embedded for security and simplicity
- Random selection ensures variety without duplicates
- The plugin is stateless and thread-safe

## Future Enhancements

Possible improvements for future versions:

- [ ] Support for filtering by cuisine type
- [ ] Support for filtering by cost range
- [ ] Support for filtering by rating
- [ ] Integration with external restaurant APIs
- [ ] User favorites and recommendations
- [ ] Location-based filtering within Barcelona

## Contributing

To contribute improvements:

1. Fork the repository
2. Make your changes
3. Test with `make test`
4. Submit a pull request

## License

This plugin is part of the Sup project and follows the same licensing terms.