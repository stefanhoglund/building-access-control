// Package httpx provides a tiny bounded-timeout JSON HTTP helper shared by
// every component's outbound calls (REQ-DIST-004: network calls shall use
// bounded timeouts). The specific timeout value is a provisional
// implementation parameter (design-specification.md section 12.7), not a
// value REQ-DIST-004 itself mandates.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultTimeout is the provisional bounded timeout used by this slice's
// components. It is not required by REQ-DIST-004 to be any particular
// value.
const DefaultTimeout = 2 * time.Second

// PostJSON POSTs body as JSON to url with a bounded timeout and decodes
// the JSON response into out (which may be nil to discard the body).
func PostJSON(url string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("httpx: marshal request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("httpx: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return do(req, out)
}

// GetJSON GETs url with a bounded timeout and decodes the JSON response
// into out.
func GetJSON(url string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("httpx: build request: %w", err)
	}
	return do(req, out)
}

// PutJSON PUTs body as JSON to url with a bounded timeout and decodes the
// JSON response into out (which may be nil to discard the body). Used by
// the real BuildSim adapter's PUT /api/doors call.
func PutJSON(url string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("httpx: marshal request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("httpx: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return do(req, out)
}

func do(req *http.Request, out any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("httpx: %s %s: %w", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("httpx: read response body: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("httpx: %s %s: status %d: %s", req.Method, req.URL, resp.StatusCode, string(data))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("httpx: decode response: %w", err)
	}
	return nil
}
