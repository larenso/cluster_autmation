package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type PorkbunClient struct {
	c      *http.Client
	url    *url.URL
	APIKey string
	Secret string
}

func NewPorkbunClient(c *http.Client, url *url.URL) *PorkbunClient {
	return &PorkbunClient{c: c, url: url}
}

func (pc *PorkbunClient) Delete(ctx context.Context, domain string, id int) error {
	url := pc.url.JoinPath("dns", "delete", domain, strconv.Itoa(id))

	body, err := pc.do(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("failed to remove a record: %w", err)
	}

	recResp := Status{}
	if err = json.Unmarshal(body, &recResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Status.Status
	return nil
}

func (pc *PorkbunClient) Create(ctx context.Context, domain string, record *Record) error {
	url := pc.url.JoinPath("dns", "create", domain)

	body, err := pc.do(ctx, url, record)
	if err != nil {
		return fmt.Errorf("failed to create a record: %w", err)
	}

	recResp := recordCreateResp{}
	if err = json.Unmarshal(body, &recResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Status.Status recResp.ID
	return nil
}

func (pc *PorkbunClient) ListRecords(ctx context.Context, domain string) ([]Record, error) {
	url := pc.url.JoinPath("dns", "retrieve", domain)

	body, err := pc.do(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	recResp := recordListResp{}
	if err = json.Unmarshal(body, &recResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return recResp.Records, nil
}

func (pc *PorkbunClient) do(ctx context.Context, url *url.URL, record *Record) ([]byte, error) {
	var err error
	var auth, body []byte
	var req *http.Request
	var resp *http.Response

	authReq := authRequest{
		APIKey:       pc.APIKey,
		SecretAPIKey: pc.Secret,
	}
	if record != nil {
		authReq = authRequest{
			APIKey:       pc.APIKey,
			SecretAPIKey: pc.Secret,
			Name:         record.Name,
			Type:         record.Type,
			Content:      record.Content,
			TTL:          record.TTL,
			Prio:         record.Prio,
			Notes:        record.Notes,
		}
	}

	if auth, err = json.Marshal(authReq); err != nil {
		return nil, fmt.Errorf("failed to marshall secrets: %w", err)
	}

	if req, err = http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(auth)); err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if resp, err = pc.c.Do(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			// todo print
		}
	}()

	maxBytesReader := io.LimitReader(resp.Body, 1024*1024) // 1Mb
	if body, err = io.ReadAll(maxBytesReader); err != nil {
		return nil, fmt.Errorf("failed to read response: %w, (code: %d)", err, resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to send response: %s", bytesToStringOrNoData(body))
	}

	return body, nil
}

func bytesToStringOrNoData(b []byte) string {
	if b == nil {
		return "no data"
	}
	return string(b)
}
