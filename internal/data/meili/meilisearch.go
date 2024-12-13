package meili

import (
	"fmt"

	"github.com/meilisearch/meilisearch-go"
)

// Client Meilisearch client
type Client struct {
	client meilisearch.ServiceManager
}

// New new Meilisearch client
func New(host, apiKey string) *Client {
	if host == "" {
		return nil
	}
	ms := meilisearch.New(host, meilisearch.WithAPIKey(apiKey))
	return &Client{client: ms}
}

// Search search from Meilisearch
func (c *Client) Search(index, query string, options *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("meilisearch client is nil, cannot perform search")
	}
	resp, err := c.client.Index(index).Search(query, options)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search error: %w", err)
	}
	return resp, nil
}

// IndexDocuments index document to Meilisearch
func (c *Client) IndexDocuments(index string, documents any, primaryKey ...string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("meilisearch client is nil, cannot index documents")
	}
	res, err := c.client.Index(index).AddDocuments(documents, primaryKey...)
	if err != nil {
		return fmt.Errorf("meilisearch index document error: %w", err)
	}
	fmt.Printf("Indexed documents with task ID: %d\n", res.TaskUID)
	return nil
}

// UpdateDocuments update document to Meilisearch
func (c *Client) UpdateDocuments(index string, documents any) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("meilisearch client is nil, cannot update documents")
	}
	res, err := c.client.Index(index).UpdateDocuments(documents)
	if err != nil {
		return fmt.Errorf("meilisearch update document error: %w", err)
	}
	fmt.Printf("Updated documents with task ID: %d\n", res.TaskUID)
	return nil
}

// DeleteDocuments delete document from Meilisearch
func (c *Client) DeleteDocuments(index string, documentID string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("meilisearch client is nil, cannot delete documents")
	}
	res, err := c.client.Index(index).DeleteDocument(documentID)
	if err != nil {
		return fmt.Errorf("meilisearch delete document error: %w", err)
	}
	fmt.Printf("Deleted document with task ID: %d\n", res.TaskUID)
	return nil
}

// GetClient get Meilisearch client
func (c *Client) GetClient() meilisearch.ServiceManager {
	return c.client
}
