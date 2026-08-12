package chatgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDecodeUsageSnapshotIncludesResetCreditSummary(t *testing.T) {
	snapshot, err := decodeUsageSnapshot([]byte(`{
		"plan_type":"pro",
		"rate_limit_reset_credits":{"available_count":2,"applicable_available_count":1}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RateLimitResetCredits == nil || snapshot.RateLimitResetCredits.AvailableCount != 2 {
		t.Fatalf("unexpected reset credits: %#v", snapshot.RateLimitResetCredits)
	}
	if snapshot.RateLimitResetCredits.ApplicableAvailableCount == nil || *snapshot.RateLimitResetCredits.ApplicableAvailableCount != 1 {
		t.Fatalf("unexpected applicable count: %#v", snapshot.RateLimitResetCredits.ApplicableAvailableCount)
	}
}

func TestRateLimitResetServiceListAndConsume(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("authorization = %q", got)
		}
		if got := r.Header.Get("ChatGPT-Account-ID"); got != "account" {
			t.Errorf("account ID = %q", got)
		}
		switch r.URL.Path {
		case "/backend-api/wham/rate-limit-reset-credits":
			return jsonResponse(`{"available_count":1,"applicable_available_count":1,"credits":[{"id":"credit-1","reset_type":"codex_rate_limits","status":"available","expires_at":1784246400}]}`), nil
		case "/backend-api/wham/rate-limit-reset-credits/consume":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["redeem_request_id"] != "request-1" || payload["credit_id"] != "credit-1" {
				t.Errorf("unexpected payload: %#v", payload)
			}
			return jsonResponse(`{"output":"reset","credit_id":"credit-1","redeem_request_id":"request-1","windows_reset":["codex"]}`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(`{"error":"not found"}`))}, nil
		}
	})}

	client := NewClient(WithAccessToken("token"), WithHTTPClient(httpClient))
	credits, err := client.Resets.List(context.Background(), "account")
	if err != nil {
		t.Fatal(err)
	}
	if credits.AvailableCount != 1 || len(credits.Credits) != 1 || credits.Credits[0].ID != "credit-1" {
		t.Fatalf("unexpected credits: %#v", credits)
	}
	result, err := client.Resets.Consume(context.Background(), "account", "request-1", "credit-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "reset" || result.CreditID != "credit-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRateLimitResetServiceFallsBackToLegacyRoutes(t *testing.T) {
	var paths []string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/backend-api/wham/rate-limit-reset-credits", "/backend-api/wham/rate-limit-reset-credits/consume":
			return statusJSONResponse(http.StatusNotFound, `{"error":{"message":"not found"}}`), nil
		case "/api/codex/rate-limit-reset-credits":
			return jsonResponse(`{"rate_limit_reset_credits":{"available_count":1,"credits":[{"id":"legacy-credit","status":"available"}]}}`), nil
		case "/api/codex/rate-limit-reset-credits/consume":
			return jsonResponse(`{"outcome":"already_redeemed","credit_id":"legacy-credit"}`), nil
		default:
			return statusJSONResponse(http.StatusNotFound, `{"error":{"message":"unexpected path"}}`), nil
		}
	})}
	client := NewClient(WithAccessToken("token"), WithHTTPClient(httpClient))
	credits, err := client.Resets.List(context.Background(), "account")
	if err != nil {
		t.Fatal(err)
	}
	if credits.AvailableCount != 1 || !credits.DetailsAvailable() || credits.Credits[0].ID != "legacy-credit" {
		t.Fatalf("unexpected credits: %#v", credits)
	}
	result, err := client.Resets.Consume(context.Background(), "account", "request-1", "legacy-credit")
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != RateLimitResetOutcomeAlreadyRedeemed || !result.Outcome.IsIdempotentSuccess() {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []string{
		"/backend-api/wham/rate-limit-reset-credits", "/api/codex/rate-limit-reset-credits",
		"/backend-api/wham/rate-limit-reset-credits/consume", "/api/codex/rate-limit-reset-credits/consume",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestRateLimitResetServiceFallsBackToUsageSummary(t *testing.T) {
	var paths []string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/backend-api/wham/usage" {
			return jsonResponse(`{"rate_limit_reset_credits":{"available_count":2,"applicable_available_count":1}}`), nil
		}
		return statusJSONResponse(http.StatusNotFound, `{"error":{"message":"not found"}}`), nil
	})}
	client := NewClient(WithAccessToken("token"), WithHTTPClient(httpClient))
	credits, err := client.Resets.List(context.Background(), "account")
	if err != nil {
		t.Fatal(err)
	}
	if credits.AvailableCount != 2 || credits.ApplicableAvailableCount == nil || *credits.ApplicableAvailableCount != 1 {
		t.Fatalf("unexpected summary: %#v", credits)
	}
	if credits.DetailsAvailable() {
		t.Fatal("usage-only summary must not claim that details were fetched")
	}
	want := []string{
		"/backend-api/wham/rate-limit-reset-credits",
		"/api/codex/rate-limit-reset-credits",
		"/backend-api/wham/usage",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestRateLimitResetCreditsSortAndTimestamps(t *testing.T) {
	seconds := int64(1784246400)
	millis := int64(1781654400000)
	credits := &RateLimitResetCredits{Credits: []*RateLimitResetCredit{
		{ID: "unknown", Status: "available"},
		{ID: "later", Status: "available", ExpiresAt: &seconds},
		{ID: "earlier", Status: "available", ExpiresAt: &millis},
	}}
	if got := credits.NextAvailableCredit(); got == nil || got.ID != "earlier" {
		t.Fatalf("unexpected next credit: %#v", got)
	}
	if got := credits.Credits[1].ExpiresAtMillis(); got != seconds*1000 {
		t.Fatalf("seconds conversion = %d, want %d", got, seconds*1000)
	}
	if got := credits.Credits[2].ExpiresAtMillis(); got != millis {
		t.Fatalf("millis conversion = %d, want %d", got, millis)
	}
}

func jsonResponse(body string) *http.Response {
	return statusJSONResponse(http.StatusOK, body)
}

func statusJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestNormalizeResetCreditOutcome(t *testing.T) {
	tests := map[string]string{
		"nothing_to_reset": "nothingToReset",
		"no_credit":        "noCredit",
		"already_redeemed": "alreadyRedeemed",
		"reset":            "reset",
	}
	for input, want := range tests {
		if got := string(normalizeResetCreditOutcome(input)); got != want {
			t.Fatalf("normalizeResetCreditOutcome(%q) = %q, want %q", input, got, want)
		}
	}
}
