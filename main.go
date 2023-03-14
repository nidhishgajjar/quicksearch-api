package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/nidhishgajjar/stevewozniak/auth"
)

func main() {
	app := fiber.New()

	// Set up CORS middleware with allowed origins
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3001",
		AllowMethods: "GET,POST",
		AllowHeaders: "Content-Type,Authorization",
		MaxAge:       10800,
	}))

	app.Get("/search", auth.SearchAuthenticate, func(c *fiber.Ctx) error {
		query := c.Query("q")
		language := c.Query("lang")
		handleSearch(query, language, c)
		return nil
	})

	app.Get("/sources", auth.SourcesAuthenticate, func(c *fiber.Ctx) error {
		query := c.Query("q")
		language := c.Query("lang")
		saveSearchResults(query, language, c)
		return nil
	})

	err := app.Listen(":3000")

	if err != nil {
		panic(err)
	}

}
