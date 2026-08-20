package openapi

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var SpecYAML []byte

var (
	SpecJSON []byte
)

func init() {
	var body interface{}
	if err := yaml.Unmarshal(SpecYAML, &body); err != nil {
		panic(fmt.Sprintf("failed to parse embedded openapi.yaml: %v", err))
	}

	body = cleanupYAMLMap(body)

	jsonBytes, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("failed to serialize openapi.json: %v", err))
	}
	SpecJSON = jsonBytes
}

// cleanupYAMLMap converts map[interface{}]interface{} to map[string]interface{} recursively
func cleanupYAMLMap(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[fmt.Sprint(k)] = cleanupYAMLMap(v)
		}
		return m2
	case map[string]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k] = cleanupYAMLMap(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = cleanupYAMLMap(v)
		}
	}
	return i
}

// ServeYAML returns the raw OpenAPI 3.2.0 YAML specification
func ServeYAML(c *fiber.Ctx) error {
	c.Set("Content-Type", "application/yaml; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=3600")
	return c.Status(http.StatusOK).Send(SpecYAML)
}

// ServeJSON returns the OpenAPI 3.2.0 JSON specification
func ServeJSON(c *fiber.Ctx) error {
	c.Set("Content-Type", "application/json; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=3600")
	return c.Status(http.StatusOK).Send(SpecJSON)
}

// ServeDocs returns an interactive API documentation interface using Scalar
func ServeDocs(c *fiber.Ctx) error {
	html := `<!doctype html>
<html lang="en">
  <head>
    <title>Realm API Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" type="image/svg+xml" href="https://irvanma.eu.org/favicon.ico" />
    <style>
      body {
        margin: 0;
        background-color: #0d1117;
      }
    </style>
  </head>
  <body>
    <script
      id="api-reference"
      data-url="/openapi.yaml"
      data-configuration='{"theme": "saturn", "darkMode": true, "layout": "modern", "showSidebar": true}'
    ></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Cache-Control", "public, max-age=3600")
	return c.Status(http.StatusOK).SendString(html)
}
