package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

// TemplateEngine renders HTML and plain text email content from embedded templates.
type TemplateEngine struct {
	fs        embed.FS
	funcMap   template.FuncMap
	cache     map[string]*template.Template
	cacheLock sync.RWMutex
}

// NewTemplateEngine creates and initializes a TemplateEngine with embedded templates and helper functions.
func NewTemplateEngine() *TemplateEngine {
	engine := &TemplateEngine{
		fs:    templateFS,
		cache: make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"upper":      strings.ToUpper,
			"lower":      strings.ToLower,
			"trim":       strings.TrimSpace,
			"safeHTML":   func(s string) template.HTML { return template.HTML(s) },
			"formatDate": func(t time.Time, layout string) string { return t.Format(layout) },
			"year":       func() int { return time.Now().Year() },
		},
	}
	return engine
}

// Render compiles and renders the requested template with the given data.
// It returns both the rendered HTML string and an automatically generated plain-text fallback.
func (e *TemplateEngine) Render(templateName string, data any) (string, string, error) {
	tmpl, err := e.getOrParseTemplate(templateName)
	if err != nil {
		return "", "", fmt.Errorf("failed to load template %q: %w", templateName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		return "", "", fmt.Errorf("failed to execute template %q: %w", templateName, err)
	}

	htmlContent := buf.String()
	plainText := htmlToPlainText(htmlContent)

	return htmlContent, plainText, nil
}

// RenderTitle extracts and executes the title definition from the template.
func (e *TemplateEngine) RenderTitle(templateName string, data any) (string, error) {
	tmpl, err := e.getOrParseTemplate(templateName)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "title", data); err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}

func (e *TemplateEngine) getOrParseTemplate(templateName string) (*template.Template, error) {
	e.cacheLock.RLock()
	tmpl, found := e.cache[templateName]
	e.cacheLock.RUnlock()

	if found {
		return tmpl, nil
	}

	e.cacheLock.Lock()
	defer e.cacheLock.Unlock()

	// Double-check after obtaining write lock
	if tmpl, found = e.cache[templateName]; found {
		return tmpl, nil
	}

	// Always parse layout.html together with the target template
	files := []string{"templates/layout.html"}
	targetFile := fmt.Sprintf("templates/%s", templateName)
	if !strings.HasSuffix(targetFile, ".html") {
		targetFile += ".html"
	}
	files = append(files, targetFile)

	parsed, err := template.New("layout").Funcs(e.funcMap).ParseFS(e.fs, files...)
	if err != nil {
		return nil, err
	}

	e.cache[templateName] = parsed
	return parsed, nil
}

var (
	tagRegex       = regexp.MustCompile(`<[^>]*>`)
	styleRegex     = regexp.MustCompile(`(?s)<style.*?>.*?</style>`)
	headRegex      = regexp.MustCompile(`(?s)<head.*?>.*?</head>`)
	multiLineRegex = regexp.MustCompile(`\n{3,}`)
	spaceRegex     = regexp.MustCompile(`[ \t]+`)
)

// htmlToPlainText converts HTML markup to clean, readable plain text for email clients that do not render HTML.
func htmlToPlainText(html string) string {
	// Strip head & styles
	text := styleRegex.ReplaceAllString(html, "")
	text = headRegex.ReplaceAllString(text, "")

	// Format block level elements with newlines
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n\n")
	text = strings.ReplaceAll(text, "</div>", "\n")
	text = strings.ReplaceAll(text, "</h1>", "\n\n")
	text = strings.ReplaceAll(text, "</h2>", "\n\n")
	text = strings.ReplaceAll(text, "</h3>", "\n\n")
	text = strings.ReplaceAll(text, "</li>", "\n")
	text = strings.ReplaceAll(text, "</tr>", "\n")

	// Strip remaining HTML tags
	text = tagRegex.ReplaceAllString(text, "")

	// Clean excessive spaces and newlines
	text = spaceRegex.ReplaceAllString(text, " ")
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}
	text = strings.Join(cleanedLines, "\n\n")
	text = multiLineRegex.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
