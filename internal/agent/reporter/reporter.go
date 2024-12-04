package reporter

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
	"wameter/internal/agent/config"
	"wameter/internal/types"
	"wameter/internal/version"

	"go.uber.org/zap"
)

// Reporter implements Reporter interface
type Reporter struct {
	config   *config.Config
	logger   *zap.Logger
	client   *http.Client
	buffer   chan *types.MetricsData
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewReporter creates new reporter
func NewReporter(cfg *config.Config, logger *zap.Logger) *Reporter {
	// Create HTTP client with TLS config if needed
	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if cfg.Agent.Server.TLS.Enabled {
		tlsConfig, err := createTLSConfig(cfg.Agent.Server.TLS)
		if err != nil {
			logger.Error("Failed to create TLS config", zap.Error(err))
		} else {
			transport.TLSClientConfig = tlsConfig
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Agent.Server.Timeout,
	}

	return &Reporter{
		config:   cfg,
		logger:   logger,
		client:   client,
		buffer:   make(chan *types.MetricsData, 1000),
		stopChan: make(chan struct{}),
	}
}

// Start starts the reporter
func (r *Reporter) Start(ctx context.Context) error {
	r.wg.Add(1)
	go r.processLoop(ctx)
	return nil
}

// Stop stops the reporter
func (r *Reporter) Stop() error {
	close(r.stopChan)
	r.wg.Wait()
	return nil
}

// Report sends metrics data
func (r *Reporter) Report(data *types.MetricsData) error {
	select {
	case r.buffer <- data:
		return nil
	default:
		return fmt.Errorf("reporter buffer is full")
	}
}

func (r *Reporter) processLoop(ctx context.Context) {
	defer r.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case data := <-r.buffer:
			if err := r.sendData(ctx, data); err != nil {
				r.logger.Error("Failed to send metrics",
					zap.Error(err),
					zap.Time("timestamp", data.Timestamp))
			}
		}
	}
}

// sendData sends metrics data
func (r *Reporter) sendData(ctx context.Context, data *types.MetricsData) error {
	// Set agent ID
	data.AgentID = r.config.Agent.ID

	// Set version
	data.Version = version.GetInfo().Version

	// Set hostname if not set
	if data.Hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("failed to get hostname: %w", err)
		}
		data.Hostname = hostname
	}

	// Set reported at
	data.ReportedAt = time.Now()

	if data.Metrics.Network != nil {
		data.Metrics.Network.AgentID = r.config.Agent.ID
		data.Metrics.Network.Hostname = data.Hostname
		data.Metrics.Network.ReportedAt = data.ReportedAt
	}

	r.logger.Debug("Sending metrics data",
		zap.String("agent_id", data.AgentID),
		zap.String("hostname", data.Hostname),
		zap.Time("timestamp", data.Timestamp))

	// Convert to JSON
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics data: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/api/v1/metrics", r.config.Agent.Server.Address)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wameter-agent/1.0")

	// Send request
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// createTLSConfig creates TLS config
func createTLSConfig(cfg config.TLSConfig) (*tls.Config, error) {
	// Load client certificate
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
