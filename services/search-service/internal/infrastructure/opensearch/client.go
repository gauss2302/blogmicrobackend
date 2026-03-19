package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"search-service/pkg/logger"
)

type Client struct {
	*opensearch.Client
	logger *logger.Logger
}

func NewClient(osURL string, log *logger.Logger) (*Client, error) {
	u, err := url.Parse(osURL)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenSearch URL: %w", err)
	}

	cfg := opensearch.Config{
		Addresses: []string{u.String()},
		Transport: &http.Transport{},
	}

	client, err := opensearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create OpenSearch client: %w", err)
	}

	ctx := context.Background()
	res, err := client.Info(client.Info.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("opensearch ping: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensearch ping failed: %s", res.String())
	}

	log.Info("OpenSearch client connected")
	return &Client{Client: client, logger: log}, nil
}

// IndexDocument indexes a single document. id must be non-empty.
func (c *Client) IndexDocument(ctx context.Context, index, id string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	res, err := c.Client.Index(
		index,
		bytes.NewReader(data),
		c.Client.Index.WithContext(ctx),
		c.Client.Index.WithDocumentID(id),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return fmt.Errorf("index document: %s", readBody(res.Body))
	}
	return nil
}

// DeleteDocument removes a document by id.
func (c *Client) DeleteDocument(ctx context.Context, index, id string) error {
	res, err := c.Client.Delete(index, id, c.Client.Delete.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete document: %s", readBody(res.Body))
	}
	return nil
}

// DoSearch executes a search and returns the API response. Caller must close res.Body.
func (c *Client) DoSearch(ctx context.Context, index string, body io.Reader, size, from *int) (*opensearchapi.Response, error) {
	opts := []func(*opensearchapi.SearchRequest){
		c.Client.Search.WithContext(ctx),
		c.Client.Search.WithIndex(index),
		c.Client.Search.WithBody(body),
		c.Client.Search.WithIgnoreUnavailable(true),
		c.Client.Search.WithAllowPartialSearchResults(true),
	}
	if size != nil {
		opts = append(opts, c.Client.Search.WithSize(*size))
	}
	if from != nil {
		opts = append(opts, c.Client.Search.WithFrom(*from))
	}
	return c.Client.Search(opts...)
}

func readBody(r io.Reader) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	_, _ = io.Copy(&b, r)
	return b.String()
}
