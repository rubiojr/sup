package handlers

import (
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/aemet-go"
	"github.com/rubiojr/sup/bot"
	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cmd/sup/version"
	"github.com/rubiojr/sup/internal/client"
)

type MeteoHandler struct {
	bot *bot.Bot
}

func (h *MeteoHandler) Name() string {
	return "meteo"
}

func (h *MeteoHandler) Topics() []string {
	return []string{"meteo"}
}

func NewMeteoHandler(bot *bot.Bot) *MeteoHandler {
	return &MeteoHandler{
		bot: bot,
	}
}

func (h *MeteoHandler) HandleMessage(msg *events.Message) error {
	// Extract command text from the message
	var messageText string
	if msg.Message.GetConversation() != "" {
		messageText = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		messageText = msg.Message.GetExtendedTextMessage().GetText()
	}

	// Extract command arguments (skip the command prefix and handler name)
	parts := strings.Fields(messageText)
	var text string
	if len(parts) > 2 {
		text = strings.Join(parts[2:], " ")
	}

	fmt.Printf("Meteo command received from %s: %s\n", msg.Info.Chat.String(), text)

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	cityName := strings.TrimSpace(text)
	if cityName == "" {
		c.SendText(msg.Info.Chat, "🌤️ Please specify a city name. Example: .sup meteo barcelona")
		return nil
	}

	aemetClient, err := aemet.NewWithDefaults()
	if err != nil {
		fmt.Printf("Error creating AEMET client: %v\n", err)
		c.SendText(msg.Info.Chat, "🚫 Error connecting to weather service. Please make sure AEMET_API_KEY is set.")
		return fmt.Errorf("error creating AEMET client: %w", err)
	}

	forecast, err := getForecastWithRetry(aemetClient, cityName)
	if err != nil {
		fmt.Printf("Error getting forecast for %s after retries: %v\n", cityName, err)
		c.SendText(msg.Info.Chat, fmt.Sprintf("🚫 Could not find weather data for '%s'. Please check the city name.", cityName))
		return fmt.Errorf("error getting forecast: %w", err)
	}

	message := "🌤️  El tiempo hoy\n"
	message += "==============================================\n"

	if len(forecast.Prediccion.Dia) > 0 {
		day := forecast.Prediccion.Dia[0]

		skyIcon := "🌤️"
		skyDescription := ""

		// Look for sky condition in any time period
		for _, estado := range day.EstadoCielo {
			if estado.Descripcion != "" {
				skyIcon = getSkyIcon(estado.Descripcion)
				skyDescription = estado.Descripcion
				break
			}
		}

		// Fallback if no sky description found
		if skyDescription == "" {
			skyDescription = "Tiempo variable"
		}

		tempRange := ""
		if day.Temperatura.Maxima != 0 && day.Temperatura.Minima != 0 {
			tempEmoji := getTempEmoji(day.Temperatura.Maxima)
			tempRange = fmt.Sprintf(" %d°C-%d°C %s", day.Temperatura.Minima, day.Temperatura.Maxima, tempEmoji)
		}

		windInfo := ""
		// Look for wind info in any time period
		for _, viento := range day.Viento {
			if viento.Velocidad > 0 {
				windIcon := getWindIcon(viento.Direccion)
				windInfo = fmt.Sprintf(" %d km/h %s", viento.Velocidad, windIcon)
				break
			}
		}

		message += fmt.Sprintf("%s %s: %s%s%s%s", "🗺️", forecast.Nombre, skyDescription, skyIcon, tempRange, windInfo)
	}

	err = c.SendText(msg.Info.Chat, message)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}

func getSkyIcon(description string) string {
	description = strings.ToLower(description)

	if strings.Contains(description, "despejado") || strings.Contains(description, "claro") {
		return "☀️"
	}
	if strings.Contains(description, "nube") {
		if strings.Contains(description, "poco") {
			return "🌤️"
		}
		if strings.Contains(description, "inter") {
			return "⛅"
		}
		return "☁️"
	}
	if strings.Contains(description, "lluvia") || strings.Contains(description, "chubasco") {
		return "🌧️"
	}
	if strings.Contains(description, "tormenta") {
		return "⛈️"
	}
	if strings.Contains(description, "nieve") {
		return "🌨️"
	}
	if strings.Contains(description, "niebla") {
		return "🌫️"
	}
	if strings.Contains(description, "muy nuboso") {
		return "☁️"
	}

	return "🌤️"
}

func getForecastWithRetry(client *aemet.Client, cityName string) (*aemet.Municipality, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		forecast, err := client.GetForecastByName(cityName)
		if err == nil {
			return forecast, nil
		}

		lastErr = err
		if attempt < maxRetries-1 {
			delay := time.Duration(attempt+1) * baseDelay
			fmt.Printf("Attempt %d failed for %s, retrying in %v: %v\n", attempt+1, cityName, delay, err)
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func getTempEmoji(maxTemp int) string {
	if maxTemp < 15 {
		return "❄️"
	} else if maxTemp <= 25 {
		return "🏖️"
	} else {
		return "🔥"
	}
}

func getWindIcon(direction string) string {
	direction = strings.ToLower(direction)

	switch direction {
	case "n", "norte":
		return "⬇️"
	case "ne", "nordeste":
		return "↙️"
	case "e", "este":
		return "⬅️"
	case "se", "sudeste":
		return "↖️"
	case "s", "sur":
		return "⬆️"
	case "sw", "so", "sudoeste":
		return "↗️"
	case "w", "o", "oeste":
		return "➡️"
	case "nw", "no", "noroeste":
		return "↘️"
	default:
		return "💨"
	}
}

func (h *MeteoHandler) GetHelp() handlers.HandlerHelp {
	return handlers.HandlerHelp{
		Name:        "meteo",
		Description: "Get weather forecast for a city",
		Usage:       ".sup meteo <city>",
		Examples:    []string{".sup meteo barcelona", ".sup meteo madrid"},
		Category:    "utility",
	}
}

func (h *MeteoHandler) Version() string {
	return version.String
}
