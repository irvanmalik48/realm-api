package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/router"
	"gopkg.in/yaml.v3"
)

func TestOpenAPI_Endpoints(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
		StorageDir:  t.TempDir(),
	}
	app := router.New(cfg, nil)

	// 1. Test /openapi.yaml
	reqYAML := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	respYAML, err := app.Test(reqYAML, -1)
	if err != nil {
		t.Fatalf("failed to fetch /openapi.yaml: %v", err)
	}
	if respYAML.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /openapi.yaml, got %d", respYAML.StatusCode)
	}
	bodyYAML, _ := io.ReadAll(respYAML.Body)
	var yamlMap map[string]interface{}
	if err := yaml.Unmarshal(bodyYAML, &yamlMap); err != nil {
		t.Errorf("failed to parse /openapi.yaml response as YAML: %v", err)
	}
	if v, ok := yamlMap["openapi"].(string); !ok || !strings.HasPrefix(v, "3.2") {
		t.Errorf("expected openapi version 3.2.x, got %v", yamlMap["openapi"])
	}

	// 2. Test /openapi.json
	reqJSON := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	respJSON, err := app.Test(reqJSON, -1)
	if err != nil {
		t.Fatalf("failed to fetch /openapi.json: %v", err)
	}
	if respJSON.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /openapi.json, got %d", respJSON.StatusCode)
	}
	bodyJSON, _ := io.ReadAll(respJSON.Body)
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(bodyJSON, &jsonMap); err != nil {
		t.Errorf("failed to parse /openapi.json response as JSON: %v", err)
	}
	if v, ok := jsonMap["openapi"].(string); !ok || !strings.HasPrefix(v, "3.2") {
		t.Errorf("expected openapi version 3.2.x, got %v", jsonMap["openapi"])
	}

	// 3. Test /docs
	reqDocs := httptest.NewRequest(http.MethodGet, "/docs", nil)
	respDocs, err := app.Test(reqDocs, -1)
	if err != nil {
		t.Fatalf("failed to fetch /docs: %v", err)
	}
	if respDocs.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /docs, got %d", respDocs.StatusCode)
	}
	bodyDocs, _ := io.ReadAll(respDocs.Body)
	if !strings.Contains(string(bodyDocs), "Realm API Reference") {
		t.Errorf("expected /docs HTML to contain 'Realm API Reference'")
	}

	// 4. Test /v1/openapi.yaml
	reqV1YAML := httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil)
	respV1YAML, _ := app.Test(reqV1YAML, -1)
	if respV1YAML.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /v1/openapi.yaml, got %d", respV1YAML.StatusCode)
	}
}
