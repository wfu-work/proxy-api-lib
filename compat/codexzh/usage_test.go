package codexzh_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wfu-work/proxy-api-lib/compat/codexzh"
)

func TestParseUsageResponse(t *testing.T) {
	stats, err := codexzh.ParseUsageResponse([]byte(`{"data":{"dailyQuota":1,"weeklyQuota":2,"todayUsed":3,"weekUsed":4,"todayCalls":5,"totalCalls":6}}`))
	if err != nil {
		t.Fatalf("parse wrapped: %v", err)
	}
	if stats.DailyQuota != 1 || stats.TotalCalls != 6 {
		t.Fatalf("stats = %#v", stats)
	}
	out, err := codexzh.ParseUsageResponse([]byte(`{"dailyQuota":1,"weeklyQuota":2,"todayUsed":3,"weekUsed":4,"todayCalls":5,"totalCalls":6}`))
	if err != nil {
		t.Fatalf("parse direct: %v", err)
	}
	if out.DailyQuota != 1 || out.WeekUsed != 4 {
		t.Fatalf("out = %#v", out)
	}
}

func TestUsageClientFetch(t *testing.T) {
	var gotPath string
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.URL.Query().Get("key")
		_, _ = w.Write([]byte(`{"data":{"dailyQuota":1,"weeklyQuota":2,"todayUsed":3,"weekUsed":4}}`))
	}))
	defer server.Close()
	client := codexzh.NewUsageClient(
		codexzh.WithUsageBaseURL(server.URL+"/usage"),
		codexzh.WithUsageHTTPClient(server.Client()),
	)
	stats, err := client.Fetch(context.Background(), "secret")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gotPath != "/usage" || gotKey != "secret" {
		t.Fatalf("request path/key = %q %q", gotPath, gotKey)
	}
	if stats.DailyQuota != 1 || stats.TodayUsed != 3 {
		t.Fatalf("stats = %#v", stats)
	}
}
