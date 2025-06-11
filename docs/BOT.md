# Sup Bot Mode

Start bot mode to listen for messages and run command handlers:

```bash
sup bot
```

**With custom handler prefix:**
```bash
sup bot --handler ".mybot"
```

**Command options:**
- `--handler, -p`: Command prefix to trigger bot handlers (default: ".sup")

This starts an interactive bot that can respond to messages with various commands. By default, the bot responds to messages starting with ".sup", but you can customize this prefix.
Any message sent to you or to a group where you are present will be processed by the handlers.

**Built-in bot handlers:**
- `.sup ping` - Responds with "pong"
- `.sup meteo` - [Aemet](https://www.aemet.es/) weather forecast
- `.sup help` - Shows available commands
