package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func NewClient(osURL, username, password string, log *logger.Logger) (*Client, error) {
	u, err := url.Parse(osURL)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenSearch URL: %w", err)
	}
	if username == "" || password == "" {
		return nil, fmt.Errorf("OpenSearch username and password are required")
	}

	cfg := opensearch.Config{
		Addresses: []string{u.String()},
		Username:  username,
		Password:  password,
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

// StatusError is a non-success HTTP response from OpenSearch.
type StatusError struct {
	Op     string
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %d %s", e.Op, e.Status, e.Body)
}

// IsPermanent reports whether err is an OpenSearch rejection that retrying
// cannot fix: a 4xx other than 429 Too Many Requests.
func IsPermanent(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Status >= 400 && se.Status < 500 && se.Status != http.StatusTooManyRequests
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
		return &StatusError{Op: "index document", Status: res.StatusCode, Body: readBody(res.Body)}
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
		return &StatusError{Op: "delete document", Status: res.StatusCode, Body: readBody(res.Body)}
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
