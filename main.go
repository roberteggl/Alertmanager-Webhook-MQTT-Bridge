package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const maxRequestBody = 1 << 20

func main() {
	listenAddr := getEnv("HTTP_LISTEN_ADDR", ":8080")
	broker := getEnv("MQTT_BROKER", "tcp://mosquitto:1883")
	topic := getEnv("MQTT_TOPIC", "homelab/health")
	clientID := getEnv("MQTT_CLIENT_ID", "alertmanager-mqtt-bridge")
	client := connectMQTT(broker, clientID, strings.TrimSpace(os.Getenv("MQTT_USERNAME")), strings.TrimSpace(os.Getenv("MQTT_PASSWORD")))
	app := &service{store: newAlertStore(), policy: filterPolicy{AllowedSeverities: severitySet(os.Getenv("ALLOWED_SEVERITIES")), ExcludedIdentity: labelSet(os.Getenv("IDENTITY_EXCLUDED_LABELS"))}, publisher: mqttPublisher{client: client, topic: topic}}
	server := &http.Server{Addr: listenAddr, Handler: routes(app, client, broker, topic), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("http server listening on %s", listenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server stopped: %v", err)
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	client.Disconnect(250)
}

type mqttHealth interface{ IsConnected() bool }

func routes(app *service, client mqttHealth, broker, topic string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		code := http.StatusOK
		state := "healthy"
		if !client.IsConnected() {
			code = http.StatusServiceUnavailable
			state = "unhealthy"
		}
		writeJSON(w, code, map[string]any{"status": state, "mqtt_connected": client.IsConnected(), "broker": broker, "topic": topic})
	})
	mux.HandleFunc("/alert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(contentType, "application/json") {
			jsonError(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		defer r.Body.Close()
		var payload webhookPayload
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			jsonError(w, "invalid json payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			jsonError(w, "invalid json payload", http.StatusBadRequest)
			return
		}
		result, err := app.process(payload.Alerts)
		if err != nil {
			log.Printf("mqtt publish failed: %v", err)
			jsonError(w, "failed to publish", http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func jsonError(w http.ResponseWriter, message string, code int) {
	writeJSON(w, code, map[string]string{"error": message})
}
func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

type mqttPublisher struct {
	client mqtt.Client
	topic  string
}

func (p mqttPublisher) Publish(message snapshot) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	token := p.client.Publish(p.topic, 1, true, payload)
	token.Wait()
	return token.Error()
}
func connectMQTT(broker, clientID, username, password string) mqtt.Client {
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID).SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(2 * time.Second)
	if username != "" {
		opts.SetUsername(username).SetPassword(password)
	}
	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatalf("mqtt connect failed: %v", err)
	}
	return client
}
