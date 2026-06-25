package handlers

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/http"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed openapi.json
var openapiSpec []byte

//go:embed README.md
var readmeContent []byte

var Version = "dev"

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := h.db.WithContext(r.Context()).DB()
	if err != nil || sqlDB.PingContext(r.Context()) != nil {
		writeError(w, http.StatusInternalServerError, "database unreachable", "db_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) VersionHandler(w http.ResponseWriter, r *http.Request) {
	var dbVersion string
	h.db.WithContext(r.Context()).Raw("SELECT version()").Scan(&dbVersion)
	writeJSON(w, http.StatusOK, map[string]string{
		"version":    Version,
		"db_version": dbVersion,
	})
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html><head><title>lpwallet</title>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
  body{font-family:system-ui,sans-serif;max-width:860px;margin:2rem auto;padding:0 1.5rem;line-height:1.6;color:#1a1a1a}
  h1,h2,h3{border-bottom:1px solid #e0e0e0;padding-bottom:.3em}
  code{background:#f4f4f4;padding:.1em .3em;border-radius:3px;font-size:.9em}
  pre{background:#f4f4f4;padding:1em;border-radius:4px;overflow-x:auto}
  pre code{background:none;padding:0}
  table{border-collapse:collapse;width:100%}
  th,td{border:1px solid #ddd;padding:.5em .75em;text-align:left}
  th{background:#f4f4f4}
  a{color:#0066cc}
  hr{border:none;border-top:1px solid #e0e0e0}
</style>
</head><body>{{.}}</body></html>`))

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	var buf bytes.Buffer
	if err := md.Convert(readmeContent, &buf); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	indexTmpl.Execute(w, template.HTML(buf.String())) //nolint:errcheck
}

func (h *Handler) OpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(openapiSpec) //nolint:errcheck
}

func (h *Handler) SwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html><head>
  <title>lpwallet API</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css">
</head><body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
<script>
  SwaggerUIBundle({
    url: "/api/v1/openapi.json",
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
    layout: "BaseLayout"
  });
</script>
</body></html>`)) //nolint:errcheck
}
