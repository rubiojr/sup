// Sup WASM Plugin - Hello JavaScript Example
// A simple hello world plugin written in JavaScript demonstrating
// the basic plugin interface for the Sup WhatsApp bot framework.

function handle_message() {
  try {
    // Get input JSON from Extism
    const inputData = Host.inputString();
    if (!inputData) {
      Host.outputString(JSON.stringify({
        success: false,
        error: "No input data received"
      }));
      return 0;
    }
    
    // Parse the input JSON
    const data = JSON.parse(inputData);
    const message = data.message || "";
    const sender = data.sender || "";
    const info = data.info || {};
    const pushName = info.push_name || "Unknown";
    
    // Generate response based on message content
    let reply;
    if (!message) {
      reply = `Hello ${pushName}! How can I help you?`;
    } else {
      reply = `Hello ${pushName}! You said: ${message}`;
    }
    
    // Return success response
    const response = {
      success: true,
      reply: reply
    };
    
    Host.outputString(JSON.stringify(response));
    return 0;
    
  } catch (error) {
    // Handle any errors
    const errorResponse = {
      success: false,
      error: `Plugin error: ${error.message}`
    };
    Host.outputString(JSON.stringify(errorResponse));
    return 1;
  }
}

function get_help() {
  try {
    const helpInfo = {
      name: "hellojs",
      description: "A simple hello world plugin written in JavaScript",
      usage: ".sup hello [message]",
      examples: [
        ".sup hello",
        ".sup hello world",
        ".sup hello from JavaScript!"
      ],
      category: "examples"
    };
    
    Host.outputString(JSON.stringify(helpInfo));
    return 0;
    
  } catch (error) {
    const errorResponse = {
      name: "hello",
      description: "Error getting help",
      usage: ".sup hello",
      examples: [],
      category: "examples"
    };
    Host.outputString(JSON.stringify(errorResponse));
    return 1;
  }
}

// Export functions for Extism
module.exports = {
  handle_message,
  get_help
};
