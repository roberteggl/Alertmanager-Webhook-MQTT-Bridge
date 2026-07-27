package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"strings"
)

// alertID returns Alertmanager's fingerprint when available. Its fallback is a
// length-delimited, ordered encoding of labels so distinct maps cannot collide.
func alertID(fingerprint string, labels map[string]string, excluded map[string]struct{}) (string, bool) {
	if fingerprint = strings.TrimSpace(fingerprint); fingerprint != "" {
		return "fingerprint:" + fingerprint, true
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		if _, skip := excluded[key]; !skip {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return "", false
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, key := range keys {
		value := labels[key]
		writePart(hash, key)
		writePart(hash, value)
	}
	return "labels:" + hex.EncodeToString(hash.Sum(nil)), true
}

func writePart(hash interface{ Write([]byte) (int, error) }, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = hash.Write(size[:])
	_, _ = hash.Write([]byte(value))
}
