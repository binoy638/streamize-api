package qbittorrent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

type Option func(*Client)

func NewClient(baseURL string, username string, password string, options ...Option) *Client {
	jar, _ := cookiejar.New(nil)
	client := &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
	}

	for _, option := range options {
		option(client)
	}

	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if client.httpClient.Jar == nil {
		client.httpClient.Jar = jar
	}

	return client
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func (c *Client) AddMagnet(ctx context.Context, magnetURI string, savePath string) error {
	if strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("qBittorrent URL cannot be empty")
	}
	if strings.TrimSpace(magnetURI) == "" {
		return fmt.Errorf("magnet URI cannot be empty")
	}

	if err := c.login(ctx); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("urls", magnetURI)
	if strings.TrimSpace(savePath) != "" {
		form.Set("savepath", savePath)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create qBittorrent add request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("submit torrent to qBittorrent: %w", err)
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent add torrent failed with status %d", response.StatusCode)
	}
	if strings.EqualFold(strings.TrimSpace(string(body)), "Fails.") {
		return fmt.Errorf("qBittorrent add torrent failed")
	}

	return nil
}

func (c *Client) login(ctx context.Context) error {
	form := url.Values{}
	form.Set("username", c.username)
	form.Set("password", c.password)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create qBittorrent login request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("log in to qBittorrent: %w", err)
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent login failed with status %d", response.StatusCode)
	}
	if !strings.EqualFold(strings.TrimSpace(string(body)), "Ok.") {
		return fmt.Errorf("qBittorrent login failed")
	}

	return nil
}
