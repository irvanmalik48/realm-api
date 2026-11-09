package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AbyssTelemetry holds telemetry details for the sarcastic vulnerability defense.
type AbyssTelemetry struct {
	Severity         string `json:"severity"`
	ThreatAssessment string `json:"threat_assessment"`
	Mitigation       string `json:"mitigation"`
}

// AbyssResponsePayload defines the HTTP 666 response body.
type AbyssResponsePayload struct {
	Telemetry AbyssTelemetry `json:"telemetry"`
	Error     string         `json:"error"`
	TraceID   string         `json:"trace_id"`
	Message   string         `json:"message"`
	Status    int            `json:"status"`
}

// AbyssResponse is the canonical code 666 response structure.
var AbyssResponse = AbyssResponsePayload{
	Status:  666,
	Error:   "DEVOPS_FROM_THE_ABYSS",
	TraceID: "666x-eternal-damnation-666x",
	Telemetry: AbyssTelemetry{
		Severity:         "CRITICAL_SIN",
		ThreatAssessment: "Mortal script-kiddie attempting to exploit microservices. Payload depth: laughably shallow.",
		Mitigation:       "Your IP has been containerized and deployed directly to the Seventh Circle. Do not kill the process; we are currently hot-reloading your nightmares.",
	},
	Message: "Nice endpoint scan. Unfortunately, the Prince of Darkness maintains a zero-trust architecture and your soul just failed the health check. Your local environment has been weighed, measured, and garbage-collected. Begone into the unindexed void.",
}

// Raw JSON response pre-marshaled with exact canonical key ordering and zero runtime allocations.
var abyssJSONBytes = []byte(`{"status":666,"error":"DEVOPS_FROM_THE_ABYSS","trace_id":"666x-eternal-damnation-666x","telemetry":{"severity":"CRITICAL_SIN","threat_assessment":"Mortal script-kiddie attempting to exploit microservices. Payload depth: laughably shallow.","mitigation":"Your IP has been containerized and deployed directly to the Seventh Circle. Do not kill the process; we are currently hot-reloading your nightmares."},"message":"Nice endpoint scan. Unfortunately, the Prince of Darkness maintains a zero-trust architecture and your soul just failed the health check. Your local environment has been weighed, measured, and garbage-collected. Begone into the unindexed void."}`)

var cveSignatures = []string{
	"cve-2026-109618",
	"cve-2026-109619",
	"cve_2026_109618",
	"cve_2026_109619",
	"cve2026109618",
	"cve2026109619",
	"php2shell",
	"php_session_upload_progress",
	"session.upload_progress",
}

// AbyssDefense intercepts requests probing or testing for CVE-2026-109618 / CVE-2026-109619
// and responds with HTTP status code 666.
func AbyssDefense() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if isCVEProbe(c) {
			c.Set("Content-Type", "application/json")
			c.Set("X-Trace-Id", "666x-eternal-damnation-666x")
			c.Status(666)
			return c.Send(abyssJSONBytes)
		}
		return c.Next()
	}
}

func isCVEProbe(c *fiber.Ctx) bool {
	// 1. Inspect Path & URL
	path := c.Path()
	origURL := c.OriginalURL()
	for _, sig := range cveSignatures {
		if containsFoldString(path, sig) || containsFoldString(origURL, sig) {
			return true
		}
	}

	// 2. Inspect Query String
	qs := string(c.Request().URI().QueryString())
	if qs != "" {
		for _, sig := range cveSignatures {
			if containsFoldString(qs, sig) {
				return true
			}
		}
	}

	// 3. Inspect Headers
	for key, val := range c.Request().Header.All() {
		for _, sig := range cveSignatures {
			if containsFoldBytes(key, sig) || containsFoldBytes(val, sig) {
				return true
			}
		}
	}

	// 4. Inspect Request Body (including multipart payloads, forms, JSON)
	body := c.Body()
	if len(body) > 0 {
		for _, sig := range cveSignatures {
			if containsFoldBytes(body, sig) {
				return true
			}
		}
	}

	return false
}

func containsFoldString(s, target string) bool {
	return strings.Contains(strings.ToLower(s), target)
}

func containsFoldBytes(data []byte, target string) bool {
	if len(target) == 0 {
		return true
	}
	if len(data) < len(target) {
		return false
	}
	targetLower := strings.ToLower(target)
	targetUpper := strings.ToUpper(target)
	firstL := targetLower[0]
	firstU := targetUpper[0]

	maxIdx := len(data) - len(target)
	for i := 0; i <= maxIdx; i++ {
		c := data[i]
		if c == firstL || c == firstU {
			match := true
			for j := 1; j < len(target); j++ {
				cj := data[i+j]
				if cj != targetLower[j] && cj != targetUpper[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}
