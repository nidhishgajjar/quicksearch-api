package search

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	openai "github.com/sashabaranov/go-openai"
)

type result struct {
	WebPages struct {
		Value []*struct {
			Name    string `json:"name"`
			Url     string `json:"url"`
			Snippet string `json:"snippet"`
		} `json:"value"`
	} `json:"webPages"`
}

type SearchResult struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Call the Bing API to get snippets
func GetBingResponse(query string, c *fiber.Ctx) ([]SearchResult, error) {

	// Define the Bing API endpoint and parameters
	endpoint := os.Getenv("BING_ENDPOINT")
	count := "10"
	offset := "0"
	mkt := "en-US"
	apiKey := os.Getenv("BING_API_KEY")

	// Create a URL object with the query parameters
	apiUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	parameters := url.Values{}
	parameters.Add("q", query)
	parameters.Add("count", count)
	parameters.Add("offset", offset)
	parameters.Add("mkt", mkt)
	apiUrl.RawQuery = parameters.Encode()

	// Create an HTTP GET request with the API URL and headers
	req, err := http.NewRequest("GET", apiUrl.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Ocp-Apim-Subscription-Key", apiKey)
	req.Header.Add("Accept-Encoding", "gzip")

	// Send the HTTP request and get the response
	bingClient := &http.Client{}
	resp, err := bingClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check if the response is compressed
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}

	// Parse the JSON response into a SearchResultList object
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON response into a struct
	var resultData result
	if err := json.Unmarshal(body, &resultData); err != nil {
		return nil, err
	}

	// Build the response
	searchResults := make([]SearchResult, 0, len(resultData.WebPages.Value))
	for _, value := range resultData.WebPages.Value {
		result := SearchResult{
			Name:    value.Name,
			URL:     value.Url,
			Snippet: value.Snippet,
		}
		searchResults = append(searchResults, result)
	}

	c.JSON(searchResults)

	return searchResults, nil
}

func GenerateOpenAIResponse(snippetsChannel <-chan string, query string, language string) (<-chan string, error) {
	var buf bytes.Buffer

	// Add each snippet to the buffer as it arrives
	for snippet := range snippetsChannel {
		buf.WriteString(snippet + "\n")
	}

	// Create the prompt with the summarized snippets
	prompt := "Search Results:\n" +
		buf.String() +
		"\nQuestion: " +
		query +
		"\n\n Guidelines for response:\n" +
		"- Be concise and useful\n" +
		"- Provide an extractive summary\n" +
		"- Use easy-to-understand language\n" +
		"- Language for response: " + language

	systemRole := "You are a search engine. Your job is to provide users with a concise and useful response."

	// Call the OpenAI API to generate a response
	openAIClient := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	stream, err := openAIClient.CreateChatCompletionStream(
		context.Background(),

		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemRole,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			Temperature:      0.0,
			TopP:             1.0,
			FrequencyPenalty: 0.0,
			PresencePenalty:  0.0,
			MaxTokens:        256,
		},
	)
	if err != nil {
		return nil, err
	}

	// Create a channel to stream the response messages
	messageStream := make(chan string)

	// Receive messages from the stream in a separate goroutine and send them to the channel
	go func() {
		defer close(messageStream)
		for {
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return
				}
				messageStream <- err.Error()
				return
			}
			messageStream <- response.Choices[0].Delta.Content
		}
	}()

	return messageStream, nil
}

func GenerateRelatedQuestions(finalResponse string, query string, language string, c *fiber.Ctx) error {

	// Create the prompt with the summarized snippets
	prompt := "Search query: " +
		query +
		"\n\n Extractive Summary: " +
		finalResponse +
		"\n\nTask: \n" +
		os.Getenv("TASK")

	systemRole := os.Getenv("SYSTEM_ROLE")
	// Call the OpenAI API to generate a response
	openAIClient := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	resp, err := openAIClient.CreateChatCompletion(
		context.Background(),

		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemRole,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			Temperature:      0.3,
			TopP:             1.0,
			FrequencyPenalty: 0.0,
			PresencePenalty:  0.0,
			MaxTokens:        150,
		},
	)

	if err != nil {
		return err
	}

	// Create a Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:        os.Getenv("REDIS_HOST"),
		Password:    os.Getenv("REDIS_PASSWORD"),
		DB:          0,
		PoolSize:    10,
		PoolTimeout: 30 * time.Second,
	})

	punctuations := "!?,.;:-'\" "

	// Set the cache duration to 3 hours (10800 seconds)
	cacheDuration := time.Duration(10800) * time.Second

	// Check if query is cached in Redis
	cachedResponse, err := redisClient.Get(context.Background(), language+" : "+strings.ToLower(strings.TrimSpace(strings.Trim(query, punctuations)))+" : relatedQuestions").Result()
	if err == nil {
		// Query is cached, use cached response
		return c.SendString(cachedResponse)
	} else if err != redis.Nil {
		// An error occurred while getting the value from Redis
		log.Println(err)
	}

	relatedQuestions := resp.Choices[0].Message.Content
	c.SendString(relatedQuestions)

	// Store the search results in Redis cache
	err = redisClient.Set(context.Background(), language+" : "+strings.ToLower(strings.TrimSpace(strings.Trim(query, punctuations)))+" : relatedQuestions", relatedQuestions, cacheDuration).Err()
	if err != nil {
		log.Print("Error: Unkown")
	}

	return nil
}
