# Plugin Registry Format

This document describes the JSON format used by the sup plugin registry.

## Index File Structure

The plugin registry uses a compressed JSON index file (`index.json.gz`) that contains metadata about all available plugins. The index file is accompanied by a SHA256 checksum file (`index.json.gz.sha256`) for integrity verification.

### Index Schema

```json
{
  "version": "1.0.0",
  "updated_at": "2024-01-15T10:30:00Z",
  "plugins": {
    "plugin-name": {
      "name": "plugin-name",
      "description": "Brief description of what the plugin does",
      "author": "Plugin Author Name",
      "home_url": "https://github.com/author/plugin-repo",
      "category": "utility",
      "tags": ["tag1", "tag2", "tag3"],
      "versions": {
        "1.0.0": {
          "version": "1.0.0",
          "release_date": "2024-01-15T10:30:00Z",
          "sha256": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
          "size": 1024000,
          "min_sup_version": "0.1.0"
        },
        "1.1.0": {
          "version": "1.1.0",
          "release_date": "2024-01-20T14:15:00Z",
          "sha256": "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
          "size": 1056000,
          "min_sup_version": "0.1.0"
        }
      },
      "latest": "1.1.0"
    }
  }
}
```

## Field Descriptions

### Index Level

- `version`: Semantic version of the index format
- `updated_at`: ISO 8601 timestamp of when the index was last updated
- `plugins`: Map of plugin names to plugin metadata

### Plugin Level

- `name`: Unique identifier for the plugin (must match the key in the plugins map)
- `description`: Human-readable description of the plugin's functionality
- `author`: Name or identifier of the plugin author/maintainer
- `home_url`: URL to the plugin's homepage, documentation, or source repository
- `category`: Category classification (e.g., "utility", "entertainment", "productivity")
- `tags`: Array of strings for additional categorization and search
- `versions`: Map of version strings to version metadata
- `latest`: Version string indicating the latest stable release

### Version Level

- `version`: Semantic version string for this release
- `release_date`: ISO 8601 timestamp of when this version was released
- `sha256`: SHA256 checksum of the plugin file for integrity verification
- `size`: Size of the plugin file in bytes
- `min_sup_version`: Minimum version of sup required to run this plugin (optional)

## Example Complete Index

```json
{
  "version": "1.0.0",
  "updated_at": "2024-01-20T15:00:00Z",
  "plugins": {
    "weather": {
      "name": "weather",
      "description": "Weather information and forecasts",
      "author": "WeatherBot Team",
      "home_url": "https://github.com/weatherbot/sup-weather-plugin",
      "category": "utility",
      "tags": ["weather", "forecast", "temperature"],
      "versions": {
        "1.0.0": {
          "version": "1.0.0",
          "release_date": "2024-01-15T10:30:00Z",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "size": 524288,
          "min_sup_version": "0.1.0"
        },
        "1.1.0": {
          "version": "1.1.0",
          "release_date": "2024-01-20T14:15:00Z",
          "sha256": "d4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666eec13ab35",
          "size": 548864,
          "min_sup_version": "0.1.0"
        }
      },
      "latest": "1.1.0"
    },
    "jokes": {
      "name": "jokes",
      "description": "Random jokes and humor generator",
      "author": "FunBot Collective",
      "home_url": "https://github.com/funbot/sup-jokes-plugin",
      "category": "entertainment",
      "tags": ["jokes", "humor", "fun"],
      "versions": {
        "0.9.0": {
          "version": "0.9.0",
          "release_date": "2024-01-10T09:00:00Z",
          "sha256": "4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce",
          "size": 327680,
          "min_sup_version": "0.1.0"
        }
      },
      "latest": "0.9.0"
    }
  }
}
```

## Registry URL Structure

The registry follows this URL pattern:

- Index file: `{registry-url}/index.json.gz`
- Index checksum: `{registry-url}/index.json.gz.sha256`
- Plugin files: `{registry-url}/plugins/{plugin-name}/{version}/{plugin-name}.wasm`

Download URLs are constructed dynamically based on the registry URL specified in commands, allowing the same index to work with different registry hosts.

## CLI Usage

### List Available Plugins
```bash
sup registry list
sup registry list --installed-only
sup registry list --available-only
```

### Download and Install Plugin
```bash
sup registry download weather
sup registry download weather 1.0.0
sup registry download jokes --registry https://custom-registry.example.com
```

### Remove Plugin
```bash
sup plugins remove weather
```

## Building a Registry Index

The `sup registry index` command can be used to build registry index files from a directory structure containing plugins.

### Directory Structure

The command expects plugins to be organized as follows:

```
plugins/
├── plugin-name/
│   ├── metadata.json (optional)
│   ├── version/
│   │   └── plugin-name.wasm
│   └── another-version/
│       └── plugin-name.wasm
└── another-plugin/
    ├── metadata.json (optional)
    └── version/
        └── another-plugin.wasm
```

### Metadata Files

Each plugin version can include an optional `metadata.json` file with plugin information:

```json
{
  "name": "plugin-name",
  "description": "Plugin description",
  "author": "Author Name",
  "home_url": "https://github.com/author/plugin",
  "category": "utility",
  "tags": ["tag1", "tag2"]
}
```

If no metadata file is provided, default values will be used.

### Usage

```bash
# Build index from plugins directory
sup registry index /path/to/plugins https://sup-registry.rbel.co

# Specify output directory
sup registry index /path/to/plugins https://sup-registry.rbel.co --output /path/to/output

# Enable verbose output
sup registry index /path/to/plugins https://sup-registry.rbel.co --verbose
```

This generates:
- `index.json` - Uncompressed JSON for debugging
- `index.json.gz` - Compressed registry index
- `index.json.gz.sha256` - Checksum file

### Example

```bash
# Create a test registry structure
mkdir -p registry/plugins/weather/1.0.0
echo "fake weather plugin data" > registry/plugins/weather/1.0.0/weather.wasm
echo '{"name":"weather","description":"Weather forecasts","author":"WeatherBot"}' > registry/plugins/weather/metadata.json

# Build the index
sup registry index registry https://sup-registry.rbel.co --output dist --verbose
```

## Security Considerations

1. **Checksum Verification**: All downloads are verified using SHA256 checksums
2. **HTTPS Only**: Registry URLs must use HTTPS for secure transmission
3. **File Integrity**: Both the index and plugin files are verified before installation
4. **Version Pinning**: Specific versions can be downloaded to ensure reproducibility