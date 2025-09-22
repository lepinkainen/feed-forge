package feed

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// TemplateGenerator handles template-based feed generation
type TemplateGenerator struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// TemplateData represents the data structure passed to feed templates
type TemplateData struct {
	// Feed metadata
	FeedTitle       string
	FeedLink        string
	FeedDescription string
	FeedAuthor      string
	FeedID          string
	Updated         string
	Generator       string

	// Items
	Items []TemplateItem

	// OpenGraph data map (URL -> OpenGraph data)
	OpenGraphData map[string]*opengraph.Data
}

// TemplateItem represents a feed item for template rendering
type TemplateItem struct {
	Title        string
	Link         string
	CommentsLink string
	ID           string
	Updated      string
	Published    string
	Author       string
	AuthorURI    string
	Categories   []string
	Score        int
	Comments     int
	Content      string
	Summary      string
	ImageURL     string
	Subreddit    string // Reddit-specific
	Domain       string // HN-specific
}

// NewTemplateGenerator creates a new template-based feed generator
func NewTemplateGenerator() *TemplateGenerator {
	return &TemplateGenerator{
		templates: make(map[string]*template.Template),
		funcMap:   TemplateFuncs(),
	}
}

// LoadTemplate loads a template from file with the given name
func (tg *TemplateGenerator) LoadTemplate(name, filePath string) error {
	slog.Debug("Loading template", "name", name, "path", filePath)

	// Read template content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", filePath, err)
	}

	// Parse template with the specified name
	tmpl, err := template.New(name).Funcs(tg.funcMap).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", filePath, err)
	}

	tg.templates[name] = tmpl
	slog.Debug("Template loaded successfully", "name", name)
	return nil
}

// LoadTemplatesFromDir loads all templates from a directory
func (tg *TemplateGenerator) LoadTemplatesFromDir(dir string) error {
	slog.Debug("Loading templates from directory", "dir", dir)

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		// Use filename without extension as template name
		name := strings.TrimSuffix(info.Name(), ".tmpl")
		return tg.LoadTemplate(name, path)
	})
}

// GenerateFromTemplate generates a feed using the specified template
func (tg *TemplateGenerator) GenerateFromTemplate(templateName string, data *TemplateData, writer io.Writer) error {
	tmpl, exists := tg.templates[templateName]
	if !exists {
		return fmt.Errorf("template %s not found", templateName)
	}

	slog.Debug("Executing template", "name", templateName, "items", len(data.Items))

	err := tmpl.Execute(writer, data)
	if err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	slog.Debug("Template executed successfully", "name", templateName)
	return nil
}

// GetAvailableTemplates returns a list of loaded template names
func (tg *TemplateGenerator) GetAvailableTemplates() []string {
	templates := make([]string, 0, len(tg.templates))
	for name := range tg.templates {
		templates = append(templates, name)
	}
	return templates
}
