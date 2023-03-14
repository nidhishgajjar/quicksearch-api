package auth

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

func SearchAuthenticate(c *fiber.Ctx) error {
	apiKey := c.Get("Authorization")

	if apiKey == "v9lqQQpYMZGRoMzShp4NjH1XzbPxRQKMI3LJLsAwoNw" {
		return c.Next()
	} else {
		return c.SendStatus(http.StatusUnauthorized)
	}
}

func SourcesAuthenticate(c *fiber.Ctx) error {
	apiKey := c.Get("Authorization")

	if apiKey == "PRx-udH7ryabqtmXNNi_Ece6Gme0zK5YSFEB5GmDJss" {
		return c.Next()
	} else {
		return c.SendStatus(http.StatusUnauthorized)
	}
}
