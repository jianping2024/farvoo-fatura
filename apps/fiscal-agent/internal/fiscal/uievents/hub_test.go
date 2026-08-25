package uievents

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNotifyBillDraftsChangedReachesSSEClient(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeSSE))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type %q", ct)
	}

	done := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		var event, data string
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "event: ") {
				event = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				data = strings.TrimPrefix(line, "data: ")
			}
			if event == EventBillDraftsChanged && data != "" {
				done <- data
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	h.NotifyBillDraftsChanged(BillDraftsChangedPayload{
		OpenCount: 2, TableDisplayName: "018", Kind: "upsert",
	})

	select {
	case data := <-done:
		if !strings.Contains(data, `"open_count":2`) || !strings.Contains(data, `018`) {
			t.Fatalf("payload %s", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}
