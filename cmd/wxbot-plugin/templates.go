package main

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed templates/*
//go:embed templates/frontend/*
//go:embed templates/frontend/src/*
//go:embed templates/process/*
var templatesFS embed.FS

type scaffoldTemplate struct {
	output   string
	template string
	frontend bool
}

var scaffoldTemplates = []scaffoldTemplate{
	{output: "go.mod", template: "templates/go.mod.tmpl"},
	{output: "manifest.json", template: "templates/manifest.json.tmpl"},
	{output: "settings.schema.json", template: "templates/settings.schema.json.tmpl"},
	{output: "wxbot-plugin.yaml", template: "templates/wxbot-plugin.yaml.tmpl"},
	{output: ".wxpluginignore", template: "templates/wxpluginignore.tmpl"},
	{output: "README.md", template: "templates/README.md.tmpl"},
	{output: "process/main.go", template: "templates/process/main.go.tmpl"},
	{output: "frontend/package.json", template: "templates/frontend/package.json.tmpl", frontend: true},
	{output: "frontend/index.html", template: "templates/frontend/index.html.tmpl", frontend: true},
	{output: "frontend/tsconfig.json", template: "templates/frontend/tsconfig.json.tmpl", frontend: true},
	{output: "frontend/vite.config.ts", template: "templates/frontend/vite.config.ts.tmpl", frontend: true},
	{output: "frontend/src/main.tsx", template: "templates/frontend/src/main.tsx.tmpl", frontend: true},
	{output: "frontend/src/App.tsx", template: "templates/frontend/src/App.tsx.tmpl", frontend: true},
	{output: "frontend/src/styles.css", template: "templates/frontend/src/styles.css.tmpl", frontend: true},
}

func pluginTemplateFiles(opts initOptions) (map[string]string, error) {
	files := map[string]string{}
	for _, item := range scaffoldTemplates {
		if item.frontend && !opts.Frontend {
			continue
		}
		body, err := renderScaffoldTemplate(item.template, opts)
		if err != nil {
			return nil, err
		}
		files[item.output] = body
	}
	return files, nil
}

func renderScaffoldTemplate(path string, opts initOptions) (string, error) {
	raw, err := templatesFS.ReadFile(path)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(path).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, opts); err != nil {
		return "", err
	}
	return out.String(), nil
}
