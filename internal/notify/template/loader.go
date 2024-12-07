package template

import (
	"embed"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wameter/internal/utils"

	"go.uber.org/zap"
	"golang.org/x/text/cases"
)

//go:embed email/* slack/* wechat/* dingtalk/* discord/* feishu/*
var templateFS embed.FS

// Type represents the type of notification template
type Type string

const (
	Email    Type = "email"
	Slack    Type = "slack"
	WeChat   Type = "wechat"
	DingTalk Type = "dingtalk"
	Discord  Type = "discord"
	Feishu   Type = "feishu"
)

// Loader manages notification templates
type Loader struct {
	logger     *zap.Logger
	templates  map[Type]*template.Template
	customTpls map[Type]map[string]string
	mu         sync.RWMutex
}

// NewLoader creates new template loader
func NewLoader(logger *zap.Logger) (*Loader, error) {
	loader := &Loader{
		logger:     logger,
		templates:  make(map[Type]*template.Template),
		customTpls: make(map[Type]map[string]string),
	}

	if err := loader.loadDefaultTemplates(); err != nil {
		return nil, err
	}

	return loader, nil
}

// loadDefaultTemplates loads templates from embedded filesystem
func (t *Loader) loadDefaultTemplates() error {
	for _, tplType := range []Type{
		Email,
		Slack,
		WeChat,
		DingTalk,
		Discord,
		Feishu,
	} {
		pattern := string(tplType)
		tmpl := template.New("").Funcs(templateFuncs)

		entries, err := templateFS.ReadDir(pattern)
		if err != nil {
			return fmt.Errorf("failed to read template directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			content, err := templateFS.ReadFile(filepath.Join(pattern, entry.Name()))
			if err != nil {
				return fmt.Errorf("failed to read template file %s: %w", entry.Name(), err)
			}

			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if _, err := tmpl.New(name).Parse(string(content)); err != nil {
				return fmt.Errorf("failed to parse template %s: %w", entry.Name(), err)
			}
		}

		t.templates[tplType] = tmpl
	}

	return nil
}

// SetCustomTemplate sets a custom template for a notification type
func (t *Loader) SetCustomTemplate(tplType Type, name, content string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.customTpls[tplType]; !ok {
		t.customTpls[tplType] = make(map[string]string)
	}

	tmpl := template.New(name).Funcs(templateFuncs)
	if _, err := tmpl.Parse(content); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	t.customTpls[tplType][name] = content
	return nil
}

// GetTemplate returns the template for given type and name
func (t *Loader) GetTemplate(tplType Type, name string) (*template.Template, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Check custom templates first
	if customContent, ok := t.customTpls[tplType][name]; ok {
		tmpl := template.New(name).Funcs(templateFuncs)
		if _, err := tmpl.Parse(customContent); err != nil {
			return nil, err
		}
		return tmpl, nil
	}

	// Fall back to default template
	if tmpl, ok := t.templates[tplType]; ok {
		if t := tmpl.Lookup(name); t != nil {
			return t, nil
		}
	}

	return nil, fmt.Errorf("template not found: %s/%s", tplType, name)
}

// Template functions available in all templates
var templateFuncs = template.FuncMap{
	"formatTime": func(t time.Time) string {
		return t.Format(time.RFC3339)
	},
	"formatBytes":     utils.FormatBytes,
	"formatBytesRate": utils.FormatBytesRate,
	"formatDuration": func(d time.Duration) string {
		return d.Round(time.Second).String()
	},
	"join": func(s []string, sep string) string {
		var result []string
		for _, v := range s {
			v = strings.TrimSpace(utils.NormalizeString(v))
			if v != "" {
				result = append(result, v)
			}
		}
		return strings.Join(result, sep)
	},
	"toTitle": cases.Title,
}
