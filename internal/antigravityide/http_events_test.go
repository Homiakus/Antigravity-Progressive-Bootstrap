package antigravityide

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientEventsPreservesAfterQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/conversations/c1/events" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.URL.Query().Get("after"); got != "7" {
			t.Fatalf("after=%q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"protocolVersion":1,"ok":true,"data":[{"seq":8,"type":"agent_delta","sourceEventId":"src1","streamKey":"c1:step:1","timestamp":"2026-08-28T11:00:00Z","payload":{"conversationId":"c1","stepIndex":1,"text":"hello","final":false}}]}`)
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, "secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Events(context.Background(), "c1", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Seq != 8 || events[0].SourceEventID != "src1" {
		t.Fatalf("events=%+v", events)
	}
}
