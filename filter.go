package main

import "strings"

type filterPolicy struct {
	AllowedSeverities map[severity]struct{}
	ExcludedIdentity  map[string]struct{}
}

func defaultFilterPolicy() filterPolicy {
	return filterPolicy{ExcludedIdentity: map[string]struct{}{}}
}

func (p filterPolicy) normalize(input alert) (normalizedAlert, string, bool) {
	parsedStatus, ok := parseStatus(input.Status)
	if !ok {
		return normalizedAlert{}, "unsupported_status", false
	}
	level := normalizeSeverity(input.Labels["severity"])
	// A resolution must not be blocked just because Alertmanager omitted a
	// severity label on it; identity is sufficient to remove the active alert.
	if parsedStatus == statusFiring && len(p.AllowedSeverities) > 0 {
		if _, ok := p.AllowedSeverities[level]; !ok {
			return normalizedAlert{}, "severity_not_allowed", false
		}
	}
	id, ok := alertID(input.Fingerprint, input.Labels, p.ExcludedIdentity)
	if !ok {
		return normalizedAlert{}, "missing_identity", false
	}
	return normalizedAlert{ID: id, Status: parsedStatus, Severity: level}, "", true
}

func severitySet(csv string) map[severity]struct{} {
	result := make(map[severity]struct{})
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result[normalizeSeverity(part)] = struct{}{}
		}
	}
	return result
}

func labelSet(csv string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, part := range strings.Split(csv, ",") {
		if part = strings.TrimSpace(part); part != "" {
			result[part] = struct{}{}
		}
	}
	return result
}
