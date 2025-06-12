# Cache System for Plugins

Sup provides a key-value caching system for plugins, allowing them to persistently store and retrieve data. The cache uses a SQLite backend with automatic expiration and is available to all WASM plugins through simple APIs.

## Overview

- **Purpose**: Store persistent data between plugin invocations
- **Implementation**: SQLite-backed key-value store
- **Isolation**: Each plugin has its own isolated storage space
- **Default Expiry**: 1 hour (configurable)
- **Usage**: Simple Get/Set operations with string keys
- **Serialization**: Values are stored as byte arrays (can store any serializable data)

## Plugin Cache API

Plugins can interact with the cache through two simple functions:

### SetCache

Stores a value in the cache with the given key.

```go
func SetCache(key string, value []byte) error
```

**Parameters**:
- `key`: String key to identify the cached item
- `value`: Binary data to store (can be strings, JSON, or any serialized data)

**Returns**:
- `error`: Error if the operation fails

### GetCache

Retrieves a value from the cache by key.

```go
func GetCache(key string) ([]byte, error)
```

**Parameters**:
- `key`: String key to look up in the cache

**Returns**:
- `[]byte`: The stored value, or nil if not found
- `error`: Error if the operation fails

## Usage Examples

### Storing Simple Values

```go
// Store a string value
err := plugin.SetCache("greeting", []byte("Hello, World!"))
if err != nil {
    return plugin.Error("Failed to store in cache: " + err.Error())
}

// Retrieve a string value
data, err := plugin.GetCache("greeting")
if err != nil {
    return plugin.Error("Failed to get from cache: " + err.Error())
}

greeting := string(data)
return plugin.Success("Retrieved: " + greeting)
```

### Storing Structured Data

```go
// Store structured data (JSON)
type User struct {
    Name  string
    Count int
    LastSeen int64
}

user := User{
    Name: "John",
    Count: 42,
    LastSeen: time.Now().Unix(),
}

userData, err := json.Marshal(user)
if err != nil {
    return plugin.Error("Failed to serialize user data: " + err.Error())
}

err = plugin.SetCache("user:john", userData)
if err != nil {
    return plugin.Error("Failed to store user data: " + err.Error())
}

// Retrieve structured data
data, err := plugin.GetCache("user:john")
if err != nil {
    return plugin.Error("Failed to get user data: " + err.Error())
}

var retrievedUser User
if err := json.Unmarshal(data, &retrievedUser); err != nil {
    return plugin.Error("Failed to parse user data: " + err.Error())
}

return plugin.Success(fmt.Sprintf("User %s has count %d",
    retrievedUser.Name, retrievedUser.Count))
```

### Creating Per-User Data

```go
// Store data specific to a WhatsApp user
userKey := fmt.Sprintf("settings:%s", input.Sender)
err := plugin.SetCache(userKey, []byte("preferred_language=en"))
```

## Best Practices

1. **Use Namespaced Keys**: Prefix keys with your plugin name or a domain to avoid conflicts
2. **Handle Missing Data**: Always check for nil values or errors when retrieving from cache
3. **Serialize Complex Data**: Use JSON or other serialization for structured data
4. **Be Resilient to Cache Misses**: Your plugin should work even if cached data expires
5. **Keep Values Small**: The cache is optimized for small values, not large files
6. **User-Specific Keys**: For user data, include the sender JID in the key

## Real-World Example: Counter Plugin

The [Counter Plugin](https://github.com/rubiojr/sup/tree/main/plugins/counter) demonstrates practical cache usage:

```go
// Store a counter in the cache
func (p *CounterPlugin) storeCount(cacheKey string, count int) error {
    countStr := strconv.Itoa(count)
    return plugin.SetCache(cacheKey, []byte(countStr))
}

// Retrieve a counter from the cache
func (p *CounterPlugin) getCurrentCount(cacheKey string) (int, error) {
    data, err := plugin.GetCache(cacheKey)
    if err != nil {
        return 0, err
    }
    if data == nil {
        return 0, fmt.Errorf("key not found in cache: %s", cacheKey)
    }

    count, err := strconv.Atoi(string(data))
    if err != nil {
        return 0, fmt.Errorf("failed to parse counter value: %w", err)
    }

    return count, nil
}
```

## Technical Details

The cache system is built on these components:

1. **Backend**: SQLite database via the `github.com/rubiojr/kv` package
2. **Expiry**: Cached items have a configurable time-to-live (default: 1 hour)
3. **Host Functions**: WASM plugins communicate with the cache through Extism host functions
4. **Sandboxing**: Plugins can only access the cache through the provided API, not directly
5. **Storage Path**: Cache data is stored in the user's sup data directory

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   WASM Plugin   │    │   Sup Plugin    │    │   SQLite        │
│   GetCache()    │───▶│   Framework     │───▶│   Database      │
│   SetCache()    │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Limitations

- **Size Limits**: Best for small data items
- **No Transactions**: Operations are atomic but can't be grouped
- **No Querying**: Simple key-value access only, no searching by value
- **No TTL Control**: Expiry time is set globally, not per item
- **No Namespacing**: Plugins must implement their own namespacing scheme

## Troubleshooting

### Common Errors

**Key not found**:
```
key not found in cache: counter:user123
```
Solution: This is normal for first-time access. Handle nil values gracefully.

**Failed to parse data**:
```
failed to parse counter value from cache: strconv.Atoi: parsing "": invalid syntax
```
Solution: Ensure consistent data types when storing and retrieving.

### Debugging Tips

1. **Check for Empty Values**: Make sure you're not storing empty byte arrays
2. **Verify Key Strings**: Typos in key names are a common source of "not found" errors
3. **Handle Serialization Errors**: Always check for errors when marshaling/unmarshaling
