package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// postJSONExpect POSTs body as JSON to url and fails the test if the
// response status does not match want.
func postJSONExpect(t *testing.T, url string, body any, want int) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("POST %s: status = %d, want %d", url, resp.StatusCode, want)
	}
}

// getJSONExpect GETs url and decodes a 200 JSON response into out.
func getJSONExpect(t *testing.T, url string, out any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response from %s: %v", url, err)
	}
}

// eventually polls check (a wall-clock concern, independent of model
// time) until it returns true, or fails the test after timeout. The
// end-to-end pipeline crosses several real HTTP hops and a polling Card
// Reader, so assertions on its effects must wait rather than assume
// synchronous completion.
func eventually(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if check() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within %s", timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
