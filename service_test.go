package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type fakePublisher struct {
	mu       sync.Mutex
	messages []snapshot
	err      error
}

func (p *fakePublisher) Publish(message snapshot) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	p.messages = append(p.messages, message)
	return nil
}

func TestServiceTransitionsAndDuplicates(t *testing.T) {
	publisher := &fakePublisher{}
	service := &service{store: newAlertStore(), policy: defaultFilterPolicy(), publisher: publisher}
	firing := alert{Fingerprint: "one", Status: "FIRING", Labels: map[string]string{"severity": "Warning"}}

	result, err := service.process([]alert{firing, firing})
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Duplicates != 1 || !result.Published {
		t.Fatalf("unexpected first result: %+v", result)
	}
	if got := publisher.messages[0]; got.State != "WARNING" || got.ActiveAlerts != 1 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}

	result, err = service.process([]alert{firing})
	if err != nil {
		t.Fatal(err)
	}
	if result.Published || result.Changed {
		t.Fatalf("duplicate delivery should be a no-op: %+v", result)
	}

	_, err = service.process([]alert{{Fingerprint: "one", Status: "resolved"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := publisher.messages[1]; got.State != "NONE" || got.ActiveAlerts != 0 {
		t.Fatalf("unexpected resolved snapshot: %+v", got)
	}
}

func TestPublishFailureIsRetried(t *testing.T) {
	publisher := &fakePublisher{err: errors.New("offline")}
	service := &service{store: newAlertStore(), policy: defaultFilterPolicy(), publisher: publisher}
	input := []alert{{Fingerprint: "one", Status: "firing", Labels: map[string]string{"severity": "critical"}}}
	if _, err := service.process(input); err == nil {
		t.Fatal("expected publish failure")
	}
	publisher.err = nil
	result, err := service.process(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Published || len(publisher.messages) != 1 {
		t.Fatalf("expected retry publish, got %+v and %d messages", result, len(publisher.messages))
	}
}

func TestIdentityIsStableAndRequiresLabels(t *testing.T) {
	first, ok := alertID("", map[string]string{"alertname": "DiskFull", "instance": "a"}, nil)
	if !ok {
		t.Fatal("expected identity")
	}
	second, ok := alertID("", map[string]string{"instance": "a", "alertname": "DiskFull"}, nil)
	if !ok || first != second {
		t.Fatalf("identity must be order-independent: %q != %q", first, second)
	}
	if _, ok := alertID("", nil, nil); ok {
		t.Fatal("empty labels must not share an identity")
	}
}

func TestHTTPAlertValidation(t *testing.T) {
	publisher := &fakePublisher{}
	app := &service{store: newAlertStore(), policy: defaultFilterPolicy(), publisher: publisher}
	handler := routes(app, disconnectedClient{}, "mqtt://test", "topic")
	request := httptest.NewRequest(http.MethodPost, "/alert", bytes.NewBufferString(`{"alerts":[]} {}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid trailing JSON to fail, got %d", response.Code)
	}
}

func TestConcurrentDelivery(t *testing.T) {
	publisher := &fakePublisher{}
	service := &service{store: newAlertStore(), policy: defaultFilterPolicy(), publisher: publisher}
	input := []alert{{Fingerprint: "one", Status: "firing", Labels: map[string]string{"severity": "error"}}}
	var group sync.WaitGroup
	for range 50 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := service.process(input); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	current, _ := service.store.needsPublish()
	if current.State != "ERROR" || current.ActiveAlerts != 1 {
		t.Fatalf("unexpected concurrent state: %+v", current)
	}
}

// disconnectedClient keeps handler tests independent of a network connection.
type disconnectedClient struct{}

func (disconnectedClient) IsConnected() bool { return false }
