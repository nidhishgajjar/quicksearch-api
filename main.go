package main

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/nidhishgajjar/stevewozniak/auth"
	"github.com/nidhishgajjar/stevewozniak/search"
)

func main() {
	app := fiber.New()

	// Set up CORS middleware with allowed origins
	app.Use(cors.New(cors.Config{
		AllowOrigins: os.Getenv("ALLOWED_ORIGINS"),
		AllowMethods: "GET,POST",
		AllowHeaders: "Content-Type,Authorization",
		MaxAge:       10800,
	}))

	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // or compress.LevelBestCompression
	}))

	app.Get("/response", auth.ResponseAuthenticate, func(c *fiber.Ctx) error {
		query := c.Query("q")
		language := c.Query("lang")
		handleSearch(query, language, c)
		return nil
	})

	app.Get("/results", auth.ResultsAuthenticate, func(c *fiber.Ctx) error {
		query := c.Query("q")
		language := c.Query("lang")
		saveSearchResults(query, language, c)
		return nil
	})

	app.Get("/search", auth.SearchAuthenticate, func(c *fiber.Ctx) error {
		query := c.Query("q")
		search.GetBingResponse(query, c)
		return nil
	})

	app.Get("/related", auth.RelatedAuthenticate, func(c *fiber.Ctx) error {
		query := c.Query("q")
		language := c.Query("lang")
		finalResponse := c.Query("finalResponse")
		search.GenerateRelatedQuestions(finalResponse, query, language, c)
		return nil
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	app.Listen(":" + port)

}
