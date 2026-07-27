package main

import "strings"

type webhookPayload struct {
	Alerts []alert `json:"alerts"`
}

type alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Fingerprint string            `json:"fingerprint"`
}

type status string

const (
	statusFiring   status = "firing"
	statusResolved status = "resolved"
)

func parseStatus(value string) (status, bool) {
	switch status(strings.ToLower(strings.TrimSpace(value))) {
	case statusFiring:
		return statusFiring, true
	case statusResolved:
		return statusResolved, true
	default:
		return "", false
	}
}

type severity string

const (
	severityNone     severity = "none"
	severityInfo     severity = "info"
	severityWarning  severity = "warning"
	severityError    severity = "error"
	severityCritical severity = "critical"
)

var severityRanks = map[severity]int{
	severityNone: 0, severityInfo: 1, severityWarning: 2, severityError: 3, severityCritical: 4,
}

func normalizeSeverity(value string) severity {
	switch severity(strings.ToLower(strings.TrimSpace(value))) {
	case severityInfo, severityWarning, severityError, severityCritical:
		return severity(strings.ToLower(strings.TrimSpace(value)))
	default:
		return severityInfo
	}
}

type normalizedAlert struct {
	ID       string
	Status   status
	Severity severity
}

type snapshot struct {
	State        string `json:"state"`
	ActiveAlerts int    `json:"active_alerts"`
	Source       string `json:"source"`
}

func emptySnapshot() snapshot { return snapshot{State: "NONE", Source: "alertmanager"} }
