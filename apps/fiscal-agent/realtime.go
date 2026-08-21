package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RealtimeNotifier implements Notifier using Supabase Realtime WebSocket.
type RealtimeNotifier struct {
	config       *config
	queue        *JobQueue
	pc           *pollController
	configPath   string
	supabaseURL  string
	restaurantID string

	mu             sync.Mutex
	conn           *websocket.Conn
	reconnectDelay time.Duration
}

// NewRealtimeNotifier creates a new Realtime notifier.
func NewRealtimeNotifier(cfg *config, queue *JobQueue, pc *pollController, configPath string) (*RealtimeNotifier, error) {
	if cfg == nil || !cfg.hasRealtimeSession() {
		return nil, fmt.Errorf("realtime session credentials missing")
	}
	supabaseURL := cfg.getSupabaseURL()
	if supabaseURL == "" {
		return nil, fmt.Errorf("cannot determine supabase_url")
	}
	if strings.TrimSpace(cfg.RestaurantID) == "" {
		return nil, fmt.Errorf("restaurant_id required for realtime")
	}

	return &RealtimeNotifier{
		config:         cfg,
		queue:          queue,
		pc:             pc,
		configPath:     configPath,
		supabaseURL:    supabaseURL,
		restaurantID:   cfg.RestaurantID,
		reconnectDelay: 2 * time.Second,
	}, nil
}

// Start begins the Realtime event loop (blocks until context canceled).
func (r *RealtimeNotifier) Start(ctx context.Context) error {
	log.Println("Realtime mode: starting")

	if err := r.compensationFetch(ctx); err != nil {
		log.Printf("Realtime: initial fetch failed: %v", err)
	}

	const maxInitialRetries = 3
	retries := 0
	connected := false
	// renew exits must always refresh — skew skip caused same-second renew loops.
	forceRefresh := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := r.connect(ctx, forceRefresh); err != nil {
			retries++
			log.Printf("Realtime: connection failed: %v (attempt %d/%d)", err, retries, maxInitialRetries)

			if !connected && retries >= maxInitialRetries {
				return fmt.Errorf("realtime connection failed after %d attempts: %w", maxInitialRetries, err)
			}

			select {
			case <-time.After(r.reconnectDelay):
				r.increaseBackoff()
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		connected = true
		retries = 0
		forceRefresh = false
		r.reconnectDelay = 2 * time.Second

		if err := r.compensationFetch(ctx); err != nil {
			log.Printf("Realtime: post-connect fetch failed: %v", err)
		}

		switch r.eventLoop(ctx) {
		case realtimeLoopRenew:
			log.Println("Realtime: access token near expiry, reconnecting to renew session")
			forceRefresh = true
		case realtimeLoopCanceled:
			return ctx.Err()
		default:
			log.Println("Realtime: connection lost, reconnecting...")
		}
	}
}

// realtimeLoopExit is why eventLoop returned (drives Start's reconnect log).
type realtimeLoopExit int

const (
	realtimeLoopLost realtimeLoopExit = iota
	realtimeLoopRenew
	realtimeLoopCanceled
)

func (r *RealtimeNotifier) connect(ctx context.Context, forceRefresh bool) error {
	log.Printf("Realtime: connect: ensuring access token (forceRefresh=%v)", forceRefresh)
	if err := r.ensureFreshAccessToken(ctx, forceRefresh); err != nil {
		return fmt.Errorf("session refresh: %w", err)
	}

	wsURL, err := r.buildWebSocketURL()
	if err != nil {
		return err
	}

	log.Println("Realtime: connect: dialing websocket")
	// Match supabase-js: apikey on the URL only; user JWT goes in phx_join (subscribe).
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.conn = conn
	r.mu.Unlock()

	log.Println("Realtime: connect: subscribing")
	if err := r.subscribe(); err != nil {
		conn.Close()
		return err
	}

	log.Println("Realtime: connected and subscribed")
	return nil
}

func (r *RealtimeNotifier) ensureFreshAccessToken(ctx context.Context, force bool) error {
	if shouldSkipAccessTokenRefresh(r.config.AccessToken, force, accessTokenRefreshSkew) {
		return nil
	}
	log.Println("Realtime: refreshing access token")
	if err := refreshSupabaseSession(ctx, r.config); err != nil {
		return err
	}
	if r.configPath != "" {
		if err := saveConfig(r.configPath, r.config); err != nil {
			log.Printf("Realtime: failed to persist refreshed tokens: %v", err)
		}
	}
	return nil
}

// shouldSkipAccessTokenRefresh is true only for non-forced connects while JWT is still inside skew.
func shouldSkipAccessTokenRefresh(accessToken string, force bool, skew time.Duration) bool {
	return !force && accessTokenUnexpired(accessToken, skew)
}

func (r *RealtimeNotifier) buildWebSocketURL() (string, error) {
	u, err := url.Parse(r.supabaseURL)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	default:
		u.Scheme = "wss"
	}
	u.Path = "/realtime/v1/websocket"

	q := u.Query()
	q.Set("apikey", r.config.AnonKey)
	q.Set("vsn", "1.0.0")
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (r *RealtimeNotifier) subscribe() error {
	// Sole Realtime join: one connection, multiple tables (print_jobs + bill_sync_jobs).
	// Do NOT open a second websocket / independent poller for bill sync.
	msg := map[string]interface{}{
		"topic": fmt.Sprintf("realtime:agent-jobs:restaurant_id=eq.%s", r.restaurantID),
		"event": "phx_join",
		"payload": map[string]interface{}{
			"config": map[string]interface{}{
				"postgres_changes": []map[string]interface{}{
					{
						"event":  "*",
						"schema": "public",
						"table":  "print_jobs",
						"filter": fmt.Sprintf("restaurant_id=eq.%s", r.restaurantID),
					},
					{
						"event":  "*",
						"schema": "public",
						"table":  "bill_sync_jobs",
						"filter": fmt.Sprintf("restaurant_id=eq.%s", r.restaurantID),
					},
				},
			},
			"access_token": r.config.AccessToken,
		},
		"ref": "1",
	}

	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no connection")
	}

	return conn.WriteJSON(msg)
}

func (r *RealtimeNotifier) eventLoop(ctx context.Context) realtimeLoopExit {
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	renewTimer := time.NewTimer(timeUntilAccessTokenRefresh(r.config.AccessToken, accessTokenRefreshSkew))
	defer renewTimer.Stop()

	messages := make(chan []byte, 10)
	errors := make(chan error, 1)

	go func() {
		for {
			_, msg, err := r.conn.ReadMessage()
			if err != nil {
				errors <- err
				return
			}
			messages <- msg
		}
	}()

	for {
		select {
		case <-ctx.Done():
			r.disconnect()
			return realtimeLoopCanceled

		case <-renewTimer.C:
			// Same path as tray restart's Realtime segment: drop WS so Start
			// re-runs ensureFreshAccessToken(force) → connect → subscribe → compensation.
			r.disconnect()
			return realtimeLoopRenew

		case <-heartbeatTicker.C:
			if err := r.sendHeartbeat(); err != nil {
				log.Printf("Realtime: heartbeat failed: %v", err)
				if err := r.compensationFetch(ctx); err != nil {
					log.Printf("Realtime: heartbeat compensation fetch failed: %v", err)
				}
				r.disconnect()
				return realtimeLoopLost
			}

		case msg := <-messages:
			r.handleMessage(msg)

		case err := <-errors:
			log.Printf("Realtime: read error: %v", err)
			r.disconnect()
			return realtimeLoopLost
		}
	}
}

func (r *RealtimeNotifier) sendHeartbeat() error {
	msg := map[string]interface{}{
		"topic":   "phoenix",
		"event":   "heartbeat",
		"payload": map[string]interface{}{},
		"ref":     fmt.Sprintf("%d", time.Now().Unix()),
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn == nil {
		return fmt.Errorf("no connection")
	}

	return r.conn.WriteJSON(msg)
}

func (r *RealtimeNotifier) scheduleOpen() bool {
	if r.pc == nil {
		return true
	}
	open, err := r.pc.scheduleOpen()
	if err != nil {
		log.Printf("Realtime: schedule error: %v", err)
		return false
	}
	return open
}

func (r *RealtimeNotifier) handleMessage(msg []byte) {
	var envelope struct {
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal(msg, &envelope); err != nil {
		log.Printf("Realtime: failed to parse message: %v", err)
		return
	}

	if envelope.Event == "postgres_changes" {
		if !r.scheduleOpen() {
			return
		}

		var payload struct {
			Data struct {
				Type   string          `json:"type"`
				Table  string          `json:"table"`
				Record json.RawMessage `json:"record"`
			} `json:"data"`
		}

		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			log.Printf("Realtime: failed to parse postgres_changes: %v", err)
			return
		}

		table := strings.TrimSpace(payload.Data.Table)
		switch table {
		case "bill_sync_jobs":
			// Doorbell only — sole ingest is PullAndIngest (same as compensation).
			go pullBillSyncsOnce(context.Background(), r.config)
			return
		case "print_jobs", "":
			// empty table: legacy payloads without table field → treat as print_jobs
		default:
			log.Printf("Realtime: ignore table %q", table)
			return
		}

		var job printJob
		if err := json.Unmarshal(payload.Data.Record, &job); err != nil {
			log.Printf("Realtime: failed to parse job record: %v", err)
			return
		}

		if job.Status != "pending" {
			return
		}

		if !r.config.jobEligibleForQueue(job) {
			return
		}

		if r.queue.Push(job) {
			logJobEnqueue("Realtime", job)
		}
	}
}

func (r *RealtimeNotifier) compensationFetch(ctx context.Context) error {
	if !r.scheduleOpen() {
		return nil
	}
	jobs, err := fetchPending(ctx, r.config.APIBase, r.config.AgentJWT)
	if err != nil {
		return err
	}

	fetched, admitted := admitPendingJobs(r.config, r.queue, jobs, "Realtime")
	if admitted > 0 {
		logCompensationSummary("Realtime", fetched, admitted)
	}
	// Same compensation round: bill sync (no second notifier).
	pullBillSyncsOnce(ctx, r.config)
	return nil
}

func (r *RealtimeNotifier) disconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.conn != nil {
		r.conn.Close()
		r.conn = nil
	}
}

func (r *RealtimeNotifier) increaseBackoff() {
	r.reconnectDelay *= 2
	if r.reconnectDelay > 60*time.Second {
		r.reconnectDelay = 60 * time.Second
	}
}

// inferSupabaseURL derives the Supabase project URL from api_base.
func inferSupabaseURL(apiBase string) string {
	if apiBase == "http://localhost:3000" || apiBase == "http://0.0.0.0:3000" {
		return "http://localhost:54321"
	}
	if apiBase == "http://127.0.0.1:3000" {
		return "http://127.0.0.1:54321"
	}

	u, err := url.Parse(apiBase)
	if err != nil {
		return ""
	}

	return u.Scheme + "://" + u.Host
}
