package replay

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type UpstreamClient struct {
	baseURL *url.URL
	client  *http.Client
}

func NewUpstreamClient(baseURL string, timeout time.Duration) (*UpstreamClient, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("upstream URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("upstream URL must include scheme and host")
	}
	return &UpstreamClient{
		baseURL: parsed,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (u *UpstreamClient) Fetch(ctx context.Context, req *http.Request, body []byte) (*http.Response, []byte, error) {
	target := *u.baseURL
	target.Path = req.URL.Path
	target.RawQuery = req.URL.RawQuery

	forwardReq, err := http.NewRequestWithContext(ctx, req.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	forwardReq.Host = u.baseURL.Host
	forwardReq.Header = cloneRequestHeaders(req.Header)

	resp, err := u.client.Do(forwardReq)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, respBody, nil
}

func cloneRequestHeaders(source http.Header) http.Header {
	cloned := source.Clone()
	stripHopByHopHeaders(cloned)
	cloned.Del("Host")
	cloned.Del("Content-Length")
	cloned.Set("Accept-Encoding", "identity")
	return cloned
}

func stripHopByHopHeaders(headers http.Header) {
	for _, header := range hopByHopHeaders {
		headers.Del(header)
	}
	if connection := headers.Get("Connection"); connection != "" {
		for _, field := range strings.Split(connection, ",") {
			headers.Del(strings.TrimSpace(field))
		}
		headers.Del("Connection")
	}
}

var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}
