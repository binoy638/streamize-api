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
			if r.Form.Get("paused") != "true" || r.Form.Get("stopped") != "true" {
				t.Fatalf("expected paused add request, got form %v", r.Form)
			}
			sawAdd = true
			return response(http.StatusOK, "Ok.", nil), nil
		default:
			return response(http.StatusNotFound, "not found", nil), nil
		}
	})

	client := NewClient("http://qbittorrent.local", "admin", "secret-password", WithHTTPClient(&http.Client{Transport: transport}))
	if err := client.AddMagnet(context.Background(), testMagnetURI, "/media/originals", true); err != nil {
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
	if err := client.AddMagnet(context.Background(), testMagnetURI, "", false); err == nil {
		t.Fatal("expected AddMagnet to return an error")
	}
}

func TestListTorrentsLogsInAndDecodesTransferState(t *testing.T) {
	var sawList bool

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			return response(http.StatusOK, "Ok.", http.Header{"Set-Cookie": []string{"SID=session; Path=/"}}), nil
		case "/api/v2/torrents/info":
			if r.Method != http.MethodGet {
				t.Fatalf("expected list method GET, got %s", r.Method)
			}
			if _, err := r.Cookie("SID"); err != nil {
				t.Fatalf("expected SID cookie on list request: %v", err)
			}
			sawList = true
			return response(http.StatusOK, `[{"hash":"0123456789ABCDEF0123456789ABCDEF01234567","name":"Test Video","size":4096,"progress":1,"state":"uploading","dlspeed":0,"upspeed":128,"num_seeds":3,"num_leechs":2,"ratio":1.5,"eta":8640000}]`, nil), nil
		default:
			return response(http.StatusNotFound, "not found", nil), nil
		}
	})

	client := NewClient("http://qbittorrent.local", "admin", "secret-password", WithHTTPClient(&http.Client{Transport: transport}))
	torrents, err := client.ListTorrents(context.Background())
	if err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	if !sawList {
		t.Fatal("expected list request")
	}
	if len(torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(torrents))
	}
	if torrents[0].Hash != "0123456789ABCDEF0123456789ABCDEF01234567" || torrents[0].Name != "Test Video" || torrents[0].Size != 4096 || torrents[0].Progress != 1 || torrents[0].State != "uploading" || torrents[0].UploadSpeed != 128 || torrents[0].NumSeeds != 3 || torrents[0].NumLeechs != 2 || torrents[0].Ratio != 1.5 {
		t.Fatalf("unexpected torrent info: %+v", torrents[0])
	}
}

func TestListTorrentFilesLogsInAndDecodesFiles(t *testing.T) {
	var sawFiles bool

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			return response(http.StatusOK, "Ok.", http.Header{"Set-Cookie": []string{"SID=session; Path=/"}}), nil
		case "/api/v2/torrents/files":
			if r.Method != http.MethodGet {
				t.Fatalf("expected files method GET, got %s", r.Method)
			}
			if r.URL.Query().Get("hash") != "0123456789abcdef0123456789abcdef01234567" {
				t.Fatalf("expected hash query, got %q", r.URL.Query().Get("hash"))
			}
			sawFiles = true
			return response(http.StatusOK, `[{"name":"movie.mkv","size":4096,"progress":0,"priority":1}]`, nil), nil
		default:
			return response(http.StatusNotFound, "not found", nil), nil
		}
	})

	client := NewClient("http://qbittorrent.local", "admin", "secret-password", WithHTTPClient(&http.Client{Transport: transport}))
	files, err := client.ListTorrentFiles(context.Background(), "0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ListTorrentFiles returned error: %v", err)
	}
	if !sawFiles {
		t.Fatal("expected files request")
	}
	if len(files) != 1 || files[0].Name != "movie.mkv" || files[0].Size != 4096 || files[0].Priority != 1 {
		t.Fatalf("unexpected torrent files: %+v", files)
	}
}

func TestDeleteTorrentLogsInAndKeepsFiles(t *testing.T) {
	var sawDelete bool

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			return response(http.StatusOK, "Ok.", http.Header{"Set-Cookie": []string{"SID=session; Path=/"}}), nil
		case "/api/v2/torrents/delete":
			if r.Method != http.MethodPost {
				t.Fatalf("expected delete method POST, got %s", r.Method)
			}
			if _, err := r.Cookie("SID"); err != nil {
				t.Fatalf("expected SID cookie on delete request: %v", err)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm returned error: %v", err)
			}
			if r.Form.Get("hashes") != "0123456789abcdef0123456789abcdef01234567" {
				t.Fatalf("expected hash, got %q", r.Form.Get("hashes"))
			}
			if r.Form.Get("deleteFiles") != "false" {
				t.Fatalf("expected deleteFiles=false, got %q", r.Form.Get("deleteFiles"))
			}
			sawDelete = true
			return response(http.StatusOK, "Ok.", nil), nil
		default:
			return response(http.StatusNotFound, "not found", nil), nil
		}
	})

	client := NewClient("http://qbittorrent.local", "admin", "secret-password", WithHTTPClient(&http.Client{Transport: transport}))
	if err := client.DeleteTorrent(context.Background(), "0123456789abcdef0123456789abcdef01234567", false); err != nil {
		t.Fatalf("DeleteTorrent returned error: %v", err)
	}
	if !sawDelete {
		t.Fatal("expected delete request")
	}
}

func TestResumeTorrentLogsInAndSubmitsHash(t *testing.T) {
	var sawResume bool

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			return response(http.StatusOK, "Ok.", http.Header{"Set-Cookie": []string{"SID=session; Path=/"}}), nil
		case "/api/v2/torrents/resume":
			if r.Method != http.MethodPost {
				t.Fatalf("expected resume method POST, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm returned error: %v", err)
			}
			if r.Form.Get("hashes") != "0123456789abcdef0123456789abcdef01234567" {
				t.Fatalf("expected hash, got %q", r.Form.Get("hashes"))
			}
			sawResume = true
			return response(http.StatusOK, "Ok.", nil), nil
		default:
			return response(http.StatusNotFound, "not found", nil), nil
		}
	})

	client := NewClient("http://qbittorrent.local", "admin", "secret-password", WithHTTPClient(&http.Client{Transport: transport}))
	if err := client.ResumeTorrent(context.Background(), "0123456789abcdef0123456789abcdef01234567"); err != nil {
		t.Fatalf("ResumeTorrent returned error: %v", err)
	}
	if !sawResume {
		t.Fatal("expected resume request")
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
