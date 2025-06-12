package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/aemet-go"
	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/cmd/sup/version"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/log"
)

type MeteoHandler struct {
	cache cache.Cache
}

func (h *MeteoHandler) Name() string {
	return "meteo"
}

func (h *MeteoHandler) Topics() []string {
	return []string{"meteo"}
}

func NewMeteoHandler(cache cache.Cache) *MeteoHandler {
	return &MeteoHandler{
		cache: cache,
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

	log.Debug("Meteo command received", "from", msg.Info.Chat.String(), "text", text)

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	cityName := strings.TrimSpace(text)
	if cityName == "" {
		c.SendText(msg.Info.Chat, "🌤️ Please specify a city name. Example: .sup meteo barcelona")
		return nil
	}

	if f := h.forecastFromCache(cityName); f != nil {
		return sendForecast(msg, c, f)
	}

	aemetClient, err := aemet.NewWithDefaults()
	if err != nil {
		log.Error("Error creating AEMET client", "error", err)
		c.SendText(msg.Info.Chat, "🚫 Error connecting to weather service. Please make sure AEMET_API_KEY is set.")
		return fmt.Errorf("error creating AEMET client: %w", err)
	}

	forecast, err := getForecastWithRetry(aemetClient, cityName)
	if err != nil {
		log.Error("Error getting forecast after retries", "city", cityName, "error", err)
		c.SendText(msg.Info.Chat, fmt.Sprintf("🚫 Could not find weather data for '%s'. Please check the city name.", cityName))
		return fmt.Errorf("error getting forecast: %w", err)
	}
	h.cacheForecast(cityName, forecast)

	return sendForecast(msg, c, forecast)
}

func (h *MeteoHandler) forecastFromCache(cityName string) *aemet.Municipality {
	cacheKey := fmt.Sprintf("%s", strings.ToLower(cityName))

	data, err := h.cache.Get([]byte(cacheKey))
	if err != nil {
		return nil
	}

	var forecast aemet.Municipality
	if err := json.Unmarshal(data, &forecast); err != nil {
		return nil
	}

	log.Debug("Forecast found in cache", "city", cityName)
	return &forecast
}

func (h *MeteoHandler) cacheForecast(cityName string, f *aemet.Municipality) {
	cacheKey := fmt.Sprintf("%s", strings.ToLower(cityName))

	data, err := json.Marshal(f)
	if err != nil {
		return
	}

	err = h.cache.Put([]byte(cacheKey), data)
	if err != nil {
		log.Debug("Failed to cache forecast", "city", cityName, "error", err)
		return
	}

	log.Debug("Cached forecast", "city", cityName)
}

func sendForecast(msg *events.Message, c *client.Client, forecast *aemet.Municipality) error {
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

	err := c.SendText(msg.Info.Chat, message)
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
			log.Warn("Attempt failed, retrying", "attempt", attempt+1, "city", cityName, "delay", delay, "error", err)
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
