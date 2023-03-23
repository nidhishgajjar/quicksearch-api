package auth

import (
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
)

func ResponseAuthenticate(c *fiber.Ctx) error {
	apiKey := c.Get("Authorization")

	if apiKey == os.Getenv("RESPONSE_API_KEY") {
		return c.Next()
	} else {
		return c.SendStatus(http.StatusUnauthorized)
	}
}

func ResultsAuthenticate(c *fiber.Ctx) error {
	apiKey := c.Get("Authorization")

	if apiKey == os.Getenv("RESULTS_API_KEY") {
		return c.Next()
	} else {
		return c.SendStatus(http.StatusUnauthorized)
	}
}

func SearchAuthenticate(c *fiber.Ctx) error {
	apiKey := c.Get("Authorization")

	if apiKey == os.Getenv("SEARCH_API_KEY") {
		return c.Next()
	} else {
		return c.SendStatus(http.StatusUnauthorized)
	}
}

func RelatedAuthenticate(c *fiber.Ctx) error {
	apiKey := c.Get("Authorization")

	if apiKey == os.Getenv("RELATED_API_KEY") {
		return c.Next()
	} else {
		return c.SendStatus(http.StatusUnauthorized)
	}
}
