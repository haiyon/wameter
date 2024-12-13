package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// SearchResponse represents the response from an Elasticsearch search query.
type SearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string          `json:"_id"`
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// Client Elasticsearch client
type Client struct {
	client *elasticsearch.Client
}

// NewClient new Elasticsearch client
func NewClient(addresses []string, username, password string) (*Client, error) {
	if len(addresses) == 0 {
		return &Client{client: nil}, nil
	}

	cfg := elasticsearch.Config{
		Addresses: addresses,
		Username:  username,
		Password:  password,
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch client creation error: %w", err)
	}

	return &Client{client: es}, nil
}

// Search search from Elasticsearch
func (c *Client) Search(ctx context.Context, indexName, query string) (*SearchResponse, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("elasticsearch client is nil, cannot perform search")
	}

	res, err := c.client.Search(
		c.client.Search.WithContext(ctx),
		c.client.Search.WithIndex(indexName),
		c.client.Search.WithBody(strings.NewReader(query)),
		c.client.Search.WithTrackTotalHits(true),
		c.client.Search.WithPretty(),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch search error: %w", err)
	}

	defer closeResponseBody(res.Body)

	var sr SearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("error parsing the response body: %w", err)
	}

	return &sr, nil
}

// IndexDocument index document to Elasticsearch
func (c *Client) IndexDocument(ctx context.Context, indexName string, documentID string, document any) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("elasticsearch client is nil, cannot index documents")
	}

	var b strings.Builder
	enc := json.NewEncoder(&b)
	if err := enc.Encode(document); err != nil {
		return fmt.Errorf("error encoding document: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: documentID,
		Body:       strings.NewReader(b.String()),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("elasticsearch indexing error: %w", err)
	}

	defer closeResponseBody(res.Body)

	if res.IsError() {
		var respBody map[string]any
		if err := json.NewDecoder(res.Body).Decode(&respBody); err != nil {
			return fmt.Errorf("error parsing the response body: %w", err)
		}
		return fmt.Errorf("elasticsearch indexing error: %s: %s", res.Status(), respBody["error"])
	}

	return nil
}

// DeleteDocument delete document from Elasticsearch
func (c *Client) DeleteDocument(ctx context.Context, indexName, documentID string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("elasticsearch client is nil, cannot delete documents")
	}

	req := esapi.DeleteRequest{
		Index:      indexName,
		DocumentID: documentID,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("elasticsearch deletion error: %w", err)
	}

	defer closeResponseBody(res.Body)

	if res.IsError() {
		var respBody map[string]any
		if err := json.NewDecoder(res.Body).Decode(&respBody); err != nil {
			return fmt.Errorf("error parsing the response body: %w", err)
		}
		return fmt.Errorf("elasticsearch deletion error: %s: %s", res.Status(), respBody["error"])
	}

	return nil
}

// GetClient get Elasticsearch client
func (c *Client) GetClient() *elasticsearch.Client {
	return c.client
}

// closeResponseBody is helper function to close response body
func closeResponseBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		fmt.Printf("Error closing response body: %v\n", err)
	}
}
