package main

import "sync"

type storedAlert struct{ Severity severity }

type transition struct {
	Snapshot                 snapshot
	Changed                  bool
	Added, Updated, Resolved int
	Duplicates               int
}

type alertStore struct {
	mu            sync.Mutex
	active        map[string]storedAlert
	desired       snapshot
	lastPublished *snapshot
}

func newAlertStore() *alertStore {
	return &alertStore{active: make(map[string]storedAlert), desired: emptySnapshot()}
}

func (s *alertStore) apply(alerts []normalizedAlert) transition {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{}, len(alerts))
	result := transition{}
	for _, alert := range alerts {
		if _, duplicate := seen[alert.ID]; duplicate {
			result.Duplicates++
			continue
		}
		seen[alert.ID] = struct{}{}
		switch alert.Status {
		case statusFiring:
			previous, exists := s.active[alert.ID]
			if !exists {
				result.Added++
			} else if previous.Severity != alert.Severity {
				result.Updated++
			}
			s.active[alert.ID] = storedAlert{Severity: alert.Severity}
		case statusResolved:
			if _, exists := s.active[alert.ID]; exists {
				delete(s.active, alert.ID)
				result.Resolved++
			}
		}
	}
	next := aggregate(s.active)
	result.Changed = next != s.desired
	s.desired = next
	result.Snapshot = next
	return result
}

func aggregate(active map[string]storedAlert) snapshot {
	if len(active) == 0 {
		return emptySnapshot()
	}
	highest := severityNone
	for _, alert := range active {
		if severityRanks[alert.Severity] > severityRanks[highest] {
			highest = alert.Severity
		}
	}
	return snapshot{State: stringUpper(highest), ActiveAlerts: len(active), Source: "alertmanager"}
}

func stringUpper(value severity) string {
	switch value {
	case severityInfo:
		return "INFO"
	case severityWarning:
		return "WARNING"
	case severityError:
		return "ERROR"
	case severityCritical:
		return "CRITICAL"
	default:
		return "NONE"
	}
}

func (s *alertStore) needsPublish() (snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.desired, s.lastPublished == nil || *s.lastPublished != s.desired
}

func (s *alertStore) markPublished(published snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.desired == published {
		copy := published
		s.lastPublished = &copy
	}
}
