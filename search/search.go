package search

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

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

func GetBingResponse(query string) ([]SearchResult, error) {

	// Define the Bing API endpoint and parameters
	endpoint := "https://api.bing.microsoft.com/v7.0/search?"
	count := "10"
	offset := "0"
	mkt := "en-US"
	apiKey := "33dde677bea54423be1d204f3432e34a"

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
	openAIClient := openai.NewClient("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")
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
