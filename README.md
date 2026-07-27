# Alertmanager-Webhook-MQTT-Bridge

Go microservice that accepts Alertmanager webhook v2 payloads and publishes a retained MQTT status message representing the highest active alert severity.

## Configuration

Environment variables:

```
HTTP_LISTEN_ADDR=:8080
MQTT_BROKER=tcp://mosquitto:1883
MQTT_TOPIC=homelab/health
MQTT_CLIENT_ID=alertmanager-mqtt-bridge
MQTT_USERNAME=your-user
MQTT_PASSWORD=your-pass
ALLOWED_SEVERITIES=warning,error,critical
IDENTITY_EXCLUDED_LABELS=prometheus,replica
```

## HTTP

- `POST /alert` with `Content-Type: application/json` (Alertmanager webhook v2 schema)
- `GET /health` returns MQTT readiness (`503` while disconnected).

Webhook bodies are limited to 1 MiB and must contain exactly one JSON value.

## Alert processing

- Only `firing` and `resolved` statuses are accepted; other alerts are ignored.
- Severity is case-insensitive. `info`, `warning`, `error`, and `critical` are supported; missing or unknown values normalize to `info`.
- The alert fingerprint is used for identity. When absent, an SHA-256 hash of canonically sorted labels is used. Alerts with neither are ignored.
- Active alerts are maintained atomically across webhooks. Duplicate deliveries are idempotent.
- A retained QoS 1 state is published only after the snapshot changes. If publishing fails, the desired snapshot remains queued and is retried by the next accepted webhook.
- `ALLOWED_SEVERITIES` optionally filters firing alerts. Resolutions are never filtered by severity, so an already-active alert can always clear.
- `IDENTITY_EXCLUDED_LABELS` is an optional comma-separated list of labels omitted from fallback identity calculation.

## MQTT

- QoS 1, retained
- Payload (JSON):

```json
{
  "state": "CRITICAL",
  "active_alerts": 3,
  "source": "alertmanager"
}
```

## Nix

Build (first build will print the required `vendorHash`):

```
nix build
```

Run:

```
nix run
```
