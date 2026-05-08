package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

type AIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // For JSON mode
}

type ResponseFormat struct {
	Type string `json:"type"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// callAI は OpenAI API (または互換プロバイダー) を叩いて結果の文字列を返します
func callAI(systemPrompt, userPrompt string, jsonMode bool) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("OPENAI_API_KEY is not set in environment or .env")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	reqBody := AIRequest{
		Model: model, // デフォルトモデル
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.7,
	}

	if jsonMode {
		reqBody.ResponseFormat = &ResponseFormat{Type: "json_object"}
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	apiURL := os.Getenv("OPENAI_API_URL")
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/chat/completions"
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API Error (Status %d): %s", resp.StatusCode, string(bodyText))
	}

	var aiResp AIResponse
	if err := json.Unmarshal(bodyText, &aiResp); err != nil {
		return "", fmt.Errorf("Failed to parse AI response: %w (Body: %s)", err, string(bodyText))
	}

	if len(aiResp.Choices) > 0 {
		return aiResp.Choices[0].Message.Content, nil
	}

	return "", errors.New("no choice returned by AI API")
}
