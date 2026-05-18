package qbittorrent

import (
	"context"
	"encoding/json"
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

type TorrentInfo struct {
	Hash          string  `json:"hash"`
	Name          string  `json:"name"`
	Size          int64   `json:"size"`
	Progress      float64 `json:"progress"`
	State         string  `json:"state"`
	DownloadSpeed int64   `json:"dlspeed"`
	UploadSpeed   int64   `json:"upspeed"`
	NumSeeds      int     `json:"num_seeds"`
	NumLeechs     int     `json:"num_leechs"`
	Ratio         float64 `json:"ratio"`
	ETA           int64   `json:"eta"`
}

type TorrentFile struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	Priority int     `json:"priority"`
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

func (c *Client) AddMagnet(ctx context.Context, magnetURI string, savePath string, paused bool) error {
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
	if paused {
		form.Set("paused", "true")
		form.Set("stopped", "true")
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

func (c *Client) ListTorrents(ctx context.Context) ([]TorrentInfo, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, fmt.Errorf("qBittorrent URL cannot be empty")
	}

	if err := c.login(ctx); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, fmt.Errorf("create qBittorrent list request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list qBittorrent torrents: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qBittorrent list torrents failed with status %d", response.StatusCode)
	}

	var torrents []TorrentInfo
	if err := json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("decode qBittorrent torrents: %w", err)
	}

	return torrents, nil
}

func (c *Client) ListTorrentFiles(ctx context.Context, hash string) ([]TorrentFile, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, fmt.Errorf("qBittorrent URL cannot be empty")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("torrent hash cannot be empty")
	}

	if err := c.login(ctx); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/torrents/files?hash="+url.QueryEscape(hash), nil)
	if err != nil {
		return nil, fmt.Errorf("create qBittorrent files request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list qBittorrent torrent files: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qBittorrent list torrent files failed with status %d", response.StatusCode)
	}

	var files []TorrentFile
	if err := json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(&files); err != nil {
		return nil, fmt.Errorf("decode qBittorrent torrent files: %w", err)
	}

	return files, nil
}

func (c *Client) DeleteTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	if strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("qBittorrent URL cannot be empty")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return fmt.Errorf("torrent hash cannot be empty")
	}

	if err := c.login(ctx); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("hashes", hash)
	form.Set("deleteFiles", fmt.Sprintf("%t", deleteFiles))

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/delete", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create qBittorrent delete request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete qBittorrent torrent: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent delete torrent failed with status %d", response.StatusCode)
	}

	return nil
}

func (c *Client) ResumeTorrent(ctx context.Context, hash string) error {
	if strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("qBittorrent URL cannot be empty")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return fmt.Errorf("torrent hash cannot be empty")
	}

	if err := c.login(ctx); err != nil {
		return err
	}

	if err := c.postTorrentHashCommand(ctx, "/api/v2/torrents/start", hash); err != nil {
		if !isEndpointMissing(err) {
			return err
		}
	} else {
		return nil
	}

	return c.postTorrentHashCommand(ctx, "/api/v2/torrents/resume", hash)
}

func (c *Client) postTorrentHashCommand(ctx context.Context, path string, hash string) error {
	form := url.Values{}
	form.Set("hashes", hash)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create qBittorrent torrent command request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("run qBittorrent torrent command: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return torrentCommandError{path: path, status: response.StatusCode}
	}

	return nil
}

type torrentCommandError struct {
	path   string
	status int
}

func (e torrentCommandError) Error() string {
	return fmt.Sprintf("qBittorrent torrent command %s failed with status %d", e.path, e.status)
}

func isEndpointMissing(err error) bool {
	commandErr, ok := err.(torrentCommandError)
	return ok && (commandErr.status == http.StatusNotFound || commandErr.status == http.StatusMethodNotAllowed)
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
