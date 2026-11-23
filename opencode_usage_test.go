package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseOpencodeUsageVariants(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		body      string
		wantCount int
		wantFirst int
		wantReset time.Time
	}{
		{
			name:      "usage wrapper with ISO reset",
			body:      `{"usage":{"rolling":{"status":"ok","percent":2,"resetsAt":"2026-11-23T04:44:55Z"},"weekly":{"status":"ok","percent":0,"resetsAt":"2026-11-30T00:00:00Z"},"monthly":{"status":"ok","percent":4,"resetsAt":"2026-12-18T07:49:58Z"}}}`,
			wantCount: 3,
			wantFirst: 2,
			wantReset: time.Date(2026, 11, 23, 4, 44, 55, 0, time.UTC),
		},
		{
			name:      "bare object with relative reset and rounded percent",
			body:      `{"rolling":{"percent":12.6,"resets_in_seconds":3600},"weekly":{"percent":50},"monthly":{"used":95,"limit":100}}`,
			wantCount: 3,
			wantFirst: 13,
			wantReset: now.Add(time.Hour),
		},
		{
			name:      "epoch milliseconds and omitted monthly",
			body:      `{"rolling":{"percent":1,"resets_at":1764000000000},"weekly":{"percent":20}}`,
			wantCount: 2,
			wantFirst: 1,
			wantReset: time.UnixMilli(1764000000000),
		},
		{
			name:      "clamps out of range percentages",
			body:      `{"usage":{"rolling":{"percent":140},"weekly":{"percent":-3}}}`,
			wantCount: 2,
			wantFirst: 100,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			usage, err := parseOpencodeUsage([]byte(tc.body), now)
			if err != nil {
				t.Fatal(err)
			}
			if !usage.Available || len(usage.Windows) != tc.wantCount {
				t.Fatalf("windows = %+v", usage.Windows)
			}
			if usage.Windows[0].Percent != tc.wantFirst {
				t.Fatalf("first percent = %d, want %d", usage.Windows[0].Percent, tc.wantFirst)
			}
			if !tc.wantReset.IsZero() && !usage.Windows[0].ResetAt.Equal(tc.wantReset) {
				t.Fatalf("reset = %v, want %v", usage.Windows[0].ResetAt, tc.wantReset)
			}
		})
	}
}

func TestParseOpencodeUsageRejectsUnknownPayload(t *testing.T) {
	for _, body := range []string{
		`not-json`,
		`[]`,
		`{"usage":{}}`,
		`{"rolling":{"status":"ok"}}`,
	} {
		if _, err := parseOpencodeUsage([]byte(body), time.Now()); err == nil {
			t.Fatalf("expected error for %q", body)
		}
	}
}

func TestParseOpencodeUsageKeepsStatus(t *testing.T) {
	usage, err := parseOpencodeUsage([]byte(`{"usage":{"rolling":{"status":"rate_limited","percent":100}}}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if usage.Windows[0].Status != "rate_limited" {
		t.Fatalf("status = %q", usage.Windows[0].Status)
	}
	if badge := opencodeBadge(usage.Windows[0]); badge != "已用尽" {
		t.Fatalf("badge = %q", badge)
	}
}

func TestFetchOpencodeUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"rolling":{"status":"ok","percent":33,"resetsAt":"2026-11-23T04:44:55Z"}}}`))
	}))
	defer server.Close()

	usage, err := fetchOpencodeUsage(context.Background(), server.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(usage.Windows) != 1 || usage.Windows[0].Percent != 33 {
		t.Fatalf("usage = %+v", usage)
	}

	if _, err := fetchOpencodeUsage(context.Background(), server.URL, "wrong-key"); err == nil || !strings.Contains(err.Error(), "密钥") {
		t.Fatalf("expected credential error, got %v", err)
	}
}

func TestFetchOpencodeUsageStatusErrors(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status == http.StatusFound {
					http.Redirect(w, r, "/elsewhere", http.StatusFound)
					return
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			if _, err := fetchOpencodeUsage(context.Background(), server.URL, "key"); err == nil {
				t.Fatalf("expected error for HTTP %d", status)
			}
		})
	}
}

func TestOpencodeStoreKeepsLastGoodUsage(t *testing.T) {
	var store opencodeUsageStore
	good := OpencodeUsage{
		Available: true,
		FetchedAt: time.Now(),
		Windows:   []OpencodeWindow{{Key: "rolling", Label: "5 小时窗口", Percent: 10}},
	}
	store.replace(good)
	store.fail(errString("boom"))
	snapshot := store.snapshot()
	if !snapshot.Available || len(snapshot.Windows) != 1 || snapshot.Err != "boom" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	snapshot.Windows[0].Percent = 99
	if store.snapshot().Windows[0].Percent != 10 {
		t.Fatal("snapshot must be a copy")
	}
}

func TestOpencodeUsageJSONShape(t *testing.T) {
	usage := OpencodeUsage{
		Available: true,
		FetchedAt: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC),
		Windows:   []OpencodeWindow{{Key: "rolling", Label: "5 小时窗口", Percent: 2}},
	}
	body, err := json.Marshal(usage)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"available":true`, `"percent":2`, `"label":"5 小时窗口"`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("JSON %s missing %s", body, want)
		}
	}
}

func TestOpencodeUsageRevision(t *testing.T) {
	reset := time.Date(2026, 11, 30, 8, 0, 0, 0, time.UTC)
	base := OpencodeUsage{
		Available: true,
		FetchedAt: time.Unix(1000, 0),
		Windows: []OpencodeWindow{
			{Key: "rolling", Percent: 20, ResetAt: reset, Status: "ok"},
			{Key: "weekly", Percent: 40, ResetAt: reset, Status: "ok"},
		},
	}
	sameContent := base
	sameContent.FetchedAt = time.Unix(2000, 0)
	if opencodeUsageRevision(base) != opencodeUsageRevision(sameContent) {
		t.Fatal("fetched_at must not change the revision")
	}
	for name, mutate := range map[string]func(*OpencodeUsage){
		"percent": func(u *OpencodeUsage) { u.Windows[0].Percent++ },
		"reset":   func(u *OpencodeUsage) { u.Windows[0].ResetAt = reset.Add(time.Hour) },
		"status":  func(u *OpencodeUsage) { u.Windows[0].Status = "limited" },
		"error":   func(u *OpencodeUsage) { u.Err = "boom" },
	} {
		changed := base
		changed.Windows = append([]OpencodeWindow(nil), base.Windows...)
		mutate(&changed)
		if opencodeUsageRevision(base) == opencodeUsageRevision(changed) {
			t.Fatalf("%s change must change the revision", name)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }
