package prompt

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

type Renderer struct {
	directory string
}

func NewRenderer(directory string) *Renderer {
	return &Renderer{directory: directory}
}

func (r *Renderer) Pair(systemFilename, userFilename string, data any) (string, string, error) {
	systemPrompt, err := r.render(systemFilename, data)
	if err != nil {
		return "", "", err
	}
	userPrompt, err := r.render(userFilename, data)
	if err != nil {
		return "", "", err
	}
	return systemPrompt, userPrompt, nil
}

func (r *Renderer) render(filename string, data any) (string, error) {
	path := filepath.Join(r.directory, filepath.Base(filename))
	tmpl, err := template.New(filepath.Base(path)).Option("missingkey=error").ParseFiles(path)
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", filename, err)
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("render prompt %s: %w", filename, err)
	}
	return output.String(), nil
}

func JSONText(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "```") {
		return value
	}
	body := strings.TrimPrefix(value, "```")
	body = strings.TrimLeftFunc(body, unicode.IsLetter)
	body = strings.TrimSuffix(strings.TrimSpace(body), "```")
	return strings.TrimSpace(body)
}
