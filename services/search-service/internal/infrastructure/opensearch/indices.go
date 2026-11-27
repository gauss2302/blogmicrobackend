package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EnsureIndices creates users and posts indices if they do not exist.
// Uses simple tokenization (standard analyzer); no language-specific stemming.
func (c *Client) EnsureIndices(ctx context.Context, usersIndex, postsIndex string) error {
	if err := c.ensureIndex(ctx, usersIndex, usersIndexMapping); err != nil {
		return fmt.Errorf("users index: %w", err)
	}
	if err := c.ensureIndex(ctx, postsIndex, postsIndexMapping); err != nil {
		return fmt.Errorf("posts index: %w", err)
	}
	return nil
}

// WaitForIndices calls EnsureIndices every delay until it succeeds, so the
// service can start before OpenSearch does. It returns ctx.Err() if ctx is
// done first.
func (c *Client) WaitForIndices(ctx context.Context, usersIndex, postsIndex string, delay time.Duration) error {
	for {
		err := c.EnsureIndices(ctx, usersIndex, postsIndex)
		if err == nil {
			c.logger.Info("OpenSearch indices ready")
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.logger.Warn(fmt.Sprintf("OpenSearch not ready, retrying in %s: %v", delay, err))
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (c *Client) ensureIndex(ctx context.Context, index string, mapping map[string]interface{}) error {
	exists, err := c.Client.Indices.Exists([]string{index}, c.Client.Indices.Exists.WithContext(ctx))
	if err != nil {
		return err
	}
	exists.Body.Close()
	if exists.StatusCode == 200 {
		return nil
	}
	body, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	res, err := c.Client.Indices.Create(
		index,
		c.Client.Indices.Create.WithContext(ctx),
		c.Client.Indices.Create.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return fmt.Errorf("create index %s: %s", index, readBody(res.Body))
	}
	return nil
}

var usersIndexMapping = map[string]interface{}{
	"settings": map[string]interface{}{
		"number_of_shards":   1,
		"number_of_replicas": 0,
		"analysis": map[string]interface{}{
			"analyzer": map[string]interface{}{
				"default": map[string]interface{}{
					"type": "standard",
				},
			},
		},
	},
	"mappings": map[string]interface{}{
		"properties": map[string]interface{}{
			"id":      map[string]string{"type": "keyword"},
			"name":    map[string]string{"type": "text", "analyzer": "standard"},
			"picture": map[string]string{"type": "keyword"},
			"bio":     map[string]string{"type": "text", "analyzer": "standard"},
		},
	},
}

var postsIndexMapping = map[string]interface{}{
	"settings": map[string]interface{}{
		"number_of_shards":   1,
		"number_of_replicas": 0,
		"analysis": map[string]interface{}{
			"analyzer": map[string]interface{}{
				"default": map[string]string{"type": "standard"},
			},
		},
	},
	"mappings": map[string]interface{}{
		"properties": map[string]interface{}{
			"id":        map[string]string{"type": "keyword"},
			"user_id":   map[string]string{"type": "keyword"},
			"title":     map[string]string{"type": "text", "analyzer": "standard"},
			"slug":      map[string]string{"type": "keyword"},
			"content":   map[string]string{"type": "text", "analyzer": "standard"},
			"published": map[string]string{"type": "boolean"},
		},
	},
}
