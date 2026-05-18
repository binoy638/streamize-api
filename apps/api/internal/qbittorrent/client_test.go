package qbittorrent

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAddMagnetLogsInAndSubmitsTorrent(t *testing.T) {
	var sawLogin bool
	var sawAdd bool

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			if r.Method != http.MethodPost {
				t.Fatalf("expected login method POST, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm returned error: %v", err)
			}
			if r.Form.Get("username") != "admin" || r.Form.Get("password") != "secret-password" {
				t.Fatalf("unexpected login credentials: %v", r.Form)
			}
			sawLogin = true
			return response(http.StatusOK, "Ok.", http.Header{"Set-Cookie": []string{"SID=session; Path=/"}}), nil
		case "/api/v2/torrents/add":
			if r.Method != http.MethodPost {
				t.Fatalf("expected add method POST, got %s", r.Method)
			}
			if _, err := r.Cookie("SID"); err != nil {
				t.Fatalf("expected SID cookie on add request: %v", err)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm returned error: %v", err)
			}
			if r.Form.Get("urls") != testMagnetURI {
				t.Fatalf("expected magnet URI %q, got %q", testMagnetURI, r.Form.Get("urls"))
			}
			if r.Form.Get("savepath") != "/media/originals" {
				t.Fatalf("expected savepath, got %q", r.Form.Get("savepath"))
			}
			sawAdd = true
			return response(http.StatusOK, "Ok.", nil), nil
		default:
			return response(http.StatusNotFound, "not found", nil), nil
		}
	})

	client := NewClient("http://qbittorrent.local", "admin", "secret-password", WithHTTPClient(&http.Client{Transport: transport}))
	if err := client.AddMagnet(context.Background(), testMagnetURI, "/media/originals"); err != nil {
		t.Fatalf("AddMagnet returned error: %v", err)
	}
	if !sawLogin {
		t.Fatal("expected login request")
	}
	if !sawAdd {
		t.Fatal("expected add request")
	}
}

func TestAddMagnetReturnsLoginFailure(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v2/auth/login" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return response(http.StatusOK, "Fails.", nil), nil
	})

	client := NewClient("http://qbittorrent.local", "admin", "bad-password", WithHTTPClient(&http.Client{Transport: transport}))
	if err := client.AddMagnet(context.Background(), testMagnetURI, ""); err == nil {
		t.Fatal("expected AddMagnet to return an error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func response(status int, body string, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}

	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

const testMagnetURI = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Test%20Video"
