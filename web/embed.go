package web

import "embed"

//go:embed templates/*.html templates/partials/*.html
var TemplateFiles embed.FS

//go:embed static/*
var StaticFiles embed.FS
