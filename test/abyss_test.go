package test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/irvanmalik48/realm-api/internal/config"
	"github.com/irvanmalik48/realm-api/internal/middleware"
	"github.com/irvanmalik48/realm-api/internal/router"
)

func TestAbyssDefense(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	app := router.New(cfg, nil)

	testCases := []struct {
		name    string
		method  string
		target  string
		headers map[string]string
		body    []byte
	}{
		{
			name:   "Path with CVE-2026-109618",
			method: http.MethodGet,
			target: "/CVE-2026-109618",
		},
		{
			name:   "Path with cve-2026-109619",
			method: http.MethodGet,
			target: "/cve-2026-109619",
		},
		{
			name:   "Path with php2shell",
			method: http.MethodGet,
			target: "/v1/php2shell",
		},
		{
			name:   "Query param with CVE-2026-109618",
			method: http.MethodGet,
			target: "/health?scan=CVE-2026-109618",
		},
		{
			name:   "Query param with php_session_upload_progress",
			method: http.MethodGet,
			target: "/?PHP_SESSION_UPLOAD_PROGRESS=evil",
		},
		{
			name:   "User-Agent header with CVE-2026-109619",
			method: http.MethodGet,
			target: "/",
			headers: map[string]string{
				"User-Agent": "Nuclei/v3 (CVE-2026-109619)",
			},
		},
		{
			name:   "Custom header with php2shell",
			method: http.MethodGet,
			target: "/health",
			headers: map[string]string{
				"X-Exploit-Test": "php2shell-probe",
			},
		},
		{
			name:   "Body with CVE-2026-109618 in JSON",
			method: http.MethodPost,
			target: "/v1/contact",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			body: []byte(`{"name":"Hacker","message":"Testing CVE-2026-109618"}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader io.Reader
			if len(tc.body) > 0 {
				bodyReader = bytes.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.target, bodyReader)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.StatusCode != 666 {
				t.Fatalf("expected HTTP status 666, got %d", resp.StatusCode)
			}

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}

			var payload middleware.AbyssResponsePayload
			if err := json.Unmarshal(respBody, &payload); err != nil {
				t.Fatalf("failed to unmarshal JSON response: %v\nBody: %s", err, string(respBody))
			}

			if payload.Status != 666 {
				t.Errorf("expected payload status 666, got %d", payload.Status)
			}
			if payload.Error != "DEVOPS_FROM_THE_ABYSS" {
				t.Errorf("expected error DEVOPS_FROM_THE_ABYSS, got %q", payload.Error)
			}
			if payload.TraceID != "666x-eternal-damnation-666x" {
				t.Errorf("expected trace_id 666x-eternal-damnation-666x, got %q", payload.TraceID)
			}
			if payload.Telemetry.Severity != "CRITICAL_SIN" {
				t.Errorf("expected severity CRITICAL_SIN, got %q", payload.Telemetry.Severity)
			}
			if payload.Telemetry.ThreatAssessment != "Mortal script-kiddie attempting to exploit microservices. Payload depth: laughably shallow." {
				t.Errorf("unexpected threat_assessment: %q", payload.Telemetry.ThreatAssessment)
			}
			if payload.Telemetry.Mitigation != "Your IP has been containerized and deployed directly to the Seventh Circle. Do not kill the process; we are currently hot-reloading your nightmares." {
				t.Errorf("unexpected mitigation: %q", payload.Telemetry.Mitigation)
			}
			expectedMsg := "Nice endpoint scan. Unfortunately, the Prince of Darkness maintains a zero-trust architecture and your soul just failed the health check. Your local environment has been weighed, measured, and garbage-collected. Begone into the unindexed void."
			if payload.Message != expectedMsg {
				t.Errorf("unexpected message: %q", payload.Message)
			}
		})
	}
}

func TestAbyssDefenseMultipart(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	app := router.New(cfg, nil)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	err := writer.WriteField("PHP_SESSION_UPLOAD_PROGRESS", "malicious_payload_trigger")
	if err != nil {
		t.Fatalf("failed to create form field: %v", err)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/storage/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 666 {
		t.Fatalf("expected HTTP status 666, got %d", resp.StatusCode)
	}
}

func TestLegitimateRequestsPass(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	app := router.New(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for legitimate request, got %d", resp.StatusCode)
	}
}
