# WhatsApp Location Handler

The WhatsApp Location handler automatically captures location messages sent to WhatsApp and stores them in AnyType with coordinates, accuracy, and metadata.

![WhatsApp Location Handler](/images/anytype-whatsapp.png)

## Overview

This handler monitors all incoming WhatsApp messages for location data. When a location message is received, it automatically extracts the coordinates, accuracy information, and sender details, then stores this data in an AnyType workspace.

## Features

- **Automatic Location Capture**: Monitors all WhatsApp messages for location data
- **AnyType Integration**: Stores locations in a structured format in AnyType
- **Metadata Preservation**: Captures sender information and accuracy data
- **Type Management**: Automatically creates the required AnyType object type if it doesn't exist

## Configuration

### Environment Variables

The handler requires two environment variables to function:

- `ANYTYPE_API_KEY`: The AnyType AppKey for authentication
- `ANYTYPE_SPACE`: The AnyType Space ID where locations will be stored

If these environment variables are not set, the handler will silently ignore location messages.

### AnyType Setup

The handler connects to AnyType running on `localhost:31009` by default. Ensure your AnyType instance is running and accessible.

## Data Structure

The handler creates a "WhatsApp Location" type in AnyType with the following fields:

- **User**: The WhatsApp user who sent the location (text field)
- **Latitude**: Geographic latitude in degrees (number field)
- **Longitude**: Geographic longitude in degrees (number field)
- **Accuracy**: Location accuracy in meters (number field)

Each location object is named "Location from {user}" and uses a location pin emoji (📍) as its icon.

## Usage

The handler operates automatically in the background. To capture locations:

1. Ensure the required environment variables are set
2. Start the sup bot with the WhatsApp Location handler enabled
3. Send or receive location messages in WhatsApp
4. Locations will be automatically stored in your AnyType workspace

## Handler Details

- **Name**: `whatsapp_location`
- **Topics**: `["*"]` (monitors all messages)
- **Category**: `storage`

## Troubleshooting

### Common Issues

1. **Locations not being stored**: Verify environment variables are set correctly
2. **AnyType connection errors**: Ensure AnyType is running on localhost:31009
3. **Type creation failures**: Check AnyType API key permissions

### Debug Mode

Enable sup bot debug logging to see detailed information about location message processing and AnyType operations.
