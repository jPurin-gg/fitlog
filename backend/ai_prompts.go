package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

func renderPrompt(filename string, data any) (string, error) {
	promptPath := filepath.Join(promptDir(), filename)
	source, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt %s: %w", filename, err)
	}

	tmpl, err := template.New(filename).Option("missingkey=error").Parse(string(source))
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt %s: %w", filename, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render prompt %s: %w", filename, err)
	}
	return buf.String(), nil
}

func renderPromptPair(systemFilename, userFilename string, data any) (string, string, error) {
	systemPrompt, err := renderPrompt(systemFilename, data)
	if err != nil {
		return "", "", err
	}
	userPrompt, err := renderPrompt(userFilename, data)
	if err != nil {
		return "", "", err
	}
	return systemPrompt, userPrompt, nil
}

func promptDir() string {
	if dir := os.Getenv("PROMPT_DIR"); dir != "" {
		return dir
	}
	return "prompts"
}
