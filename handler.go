package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nidhishgajjar/stevewozniak/search"
	"github.com/redis/go-redis/v9"
)

type streamReader struct {
	stream chan []byte
}

func (r *streamReader) Read(p []byte) (n int, err error) {
	if message, ok := <-r.stream; ok {
		n = copy(p, message)
		if n < len(message) {
			r.stream <- message[n:]
		}
		return n, nil
	}
	return 0, io.EOF
}

func handleSearch(query string, language string, c *fiber.Ctx) error {

	punctuations := "!?,.;:-'\" "

	// Create a Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:        os.Getenv("REDIS_HOST"),
		Password:    os.Getenv("REDIS_PASSWORD"),
		DB:          0,
		PoolSize:    10,
		PoolTimeout: 30 * time.Second,
	})

	// Check if query is cached in Redis
	cachedResponse, err := redisClient.Get(context.Background(), language+" : "+strings.ToLower(strings.TrimSpace(strings.Trim(query, punctuations)))).Result()
	if err == nil {
		// Query is cached, use cached response
		return c.SendString(cachedResponse)
	} else if err != redis.Nil {
		// An error occurred while getting the value from Redis
		log.Println(err)
	}

	// Query is not cached or an error occurred, fetch response from APIs

	// Create a channel to stream the snippets from Bing API to OpenAI API
	snippetsChannel := make(chan string)

	// Create a buffered channel to receive the search results
	resultsChannel := make(chan []search.SearchResult, 1)

	// Call the Bing API to get snippets concurrently and stream them to the snippets channel
	go func() {
		searchResults, err := search.GetBingResponse(query, c)
		if err != nil {
			log.Println(err)
			// return
		}
		resultsChannel <- searchResults
		for _, snippet := range searchResults {
			snippetsChannel <- snippet.Snippet
		}
		close(snippetsChannel)

	}()

	// Set the cache duration to 3 hours (10800 seconds)
	cacheDuration := time.Duration(10800) * time.Second

	// Call the OpenAI API to generate a response
	messageStream, err := search.GenerateOpenAIResponse(snippetsChannel, query, language)
	if err != nil {
		return c.SendString("Error: Generating Response please refresh try again")
	}

	// Set response headers
	c.Set("Content-Type", "text/plain")
	c.Status(fiber.StatusOK)

	// Copy the messages from messageStream to a new channel of byte slices
	byteStream := make(chan []byte)
	go func() {
		var buf bytes.Buffer
		for message := range messageStream {
			byteStream <- []byte(message)
			buf.WriteString(message)
		}

		close(byteStream)

		// Store the response in Redis cache
		err = redisClient.Set(context.Background(), language+" : "+strings.ToLower(strings.TrimSpace(strings.Trim(query, punctuations))), buf.String(), cacheDuration).Err()
		if err != nil {
			log.Print("Error: Unkown")
		}

	}()

	c.SendStream(&streamReader{byteStream})

	bingResults := <-resultsChannel
	bingResultsString, _ := json.Marshal(bingResults)

	// Store the search results in Redis cache
	err = redisClient.Set(context.Background(), language+" : "+strings.ToLower(strings.TrimSpace(strings.Trim(query, punctuations)))+" : results", bingResultsString, cacheDuration).Err()
	if err != nil {
		log.Print("Error: Unkown")
	}

	return nil
}

func saveSearchResults(query string, language string, c *fiber.Ctx) error {

	punctuations := "!?,.;:-'\" "

	// Create a Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:        os.Getenv("REDIS_HOST"),
		Password:    os.Getenv("REDIS_PASSWORD"),
		DB:          0,
		PoolSize:    10,
		PoolTimeout: 30 * time.Second,
	})

	// Retrieve the search results from Redis cache
	results, err := redisClient.Get(context.Background(), language+" : "+strings.ToLower(strings.TrimSpace(strings.Trim(query, punctuations)))+" : results").Bytes()
	if err != nil {
		if err == redis.Nil {
			// Handle the case where the key is not found in Redis cache
			return c.SendString("Error Retrieving Sources")
		}

		// Handle other errors
		log.Print("Error: Unknown" + err.Error())
	}

	resultsJSON := []search.SearchResult{}
	json.Unmarshal(results, &resultsJSON)

	c.JSON(resultsJSON)

	return nil
}
