package main

import (
	_ "embed"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/rubiojr/sup/pkg/plugin"
)

type Restaurant struct {
	Name    string
	URL     string
	Cuisine string
	Rating  int
	Cost    int
}

//go:embed bcn-restaurants.txt
var restaurantData string

type EatBcn struct{}

// Name returns the name of the plugin
func (r *EatBcn) Name() string {
	return "eat-bcn"
}

// Topics returns the topics this plugin should receive messages for
func (r *EatBcn) Topics() []string {
	return []string{"eat-bcn"}
}

func (r *EatBcn) HandleMessage(input plugin.Input) plugin.Output {
	restaurants, err := r.parseRestaurants()
	if err != nil {
		return plugin.Error("🚫 Sorry, couldn't load the restaurant list!")
	}

	if len(restaurants) < 3 {
		return plugin.Error("🚫 Not enough restaurants in the list!")
	}

	// Seed with message timestamp to get different results across invocations.
	// WASM runtimes don't provide system entropy for automatic seeding.
	rng := rand.New(rand.NewPCG(uint64(input.Info.Timestamp), uint64(len(input.Message))))

	selected := make([]Restaurant, 0, 3)
	used := make(map[int]bool)

	for len(selected) < 3 {
		idx := rng.IntN(len(restaurants))
		if !used[idx] {
			selected = append(selected, restaurants[idx])
			used[idx] = true
		}
	}

	message := "🍻 Here are 3 random Barcelona restaurant suggestions:\n\n"
	emojis := []string{"🍽️", "🥘", "🍴"}

	for i, restaurant := range selected {
		message += fmt.Sprintf(
			"%s %s\n🔗 %s\nCost: %s\nRating: %s\nCuisine: %s\n\n",
			emojis[i], restaurant.Name,
			restaurant.URL,
			r.costToEmoji(restaurant.Cost),
			r.ratingToEmoji(restaurant.Rating),
			restaurant.Cuisine,
		)
	}

	message += "¡Buen provecho! 🎉"

	return plugin.Success(message)
}

func (r *EatBcn) parseRestaurants() ([]Restaurant, error) {
	lines := strings.Split(strings.TrimSpace(restaurantData), "\n")

	if len(lines) <= 1 {
		return nil, fmt.Errorf("no restaurant data found")
	}

	var restaurants []Restaurant
	for i, line := range lines {
		if i == 0 {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "#")
		if len(parts) < 5 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		url := strings.TrimSpace(parts[1])
		cuisine := strings.TrimSpace(parts[2])

		rating, err := strconv.Atoi(strings.TrimSpace(parts[3]))
		if err != nil {
			return nil, fmt.Errorf("error parsing rating for restaurant %s: %v", name, err)
		}

		cost, err := strconv.Atoi(strings.TrimSpace(parts[4]))
		if err != nil {
			return nil, fmt.Errorf("error parsing cost for restaurant %s: %v", name, err)
		}

		restaurants = append(restaurants, Restaurant{
			Name:    name,
			URL:     url,
			Cuisine: cuisine,
			Cost:    cost,
			Rating:  rating,
		})
	}

	return restaurants, nil
}

func (r *EatBcn) costToEmoji(cost int) string {
	switch cost {
	case 0:
		return "❓"
	case 1:
		return "$"
	case 2:
		return "$$"
	case 3:
		return "$$$"
	default:
		return "❓"
	}
}

func (r *EatBcn) ratingToEmoji(rating int) string {
	switch rating {
	case 0:
		return "❓"
	case 1:
		return "⭐"
	case 2:
		return "⭐⭐"
	case 3:
		return "⭐⭐⭐"
	default:
		return "❓"
	}
}

func (r *EatBcn) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"eat-bcn",
		"Get random Barcelona restaurant suggestions",
		".sup eat-bcn",
		[]string{".sup eat-bcn"},
		"utility",
	)
}

func (r *EatBcn) GetRequiredEnvVars() []string {
	return []string{}
}

func (r *EatBcn) Version() string {
	return "0.1.0"
}

func init() {
	plugin.RegisterPlugin(&EatBcn{})
}

func main() {}
