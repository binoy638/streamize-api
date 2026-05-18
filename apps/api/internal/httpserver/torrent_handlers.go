package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

type TorrentAdder interface {
	AddMagnet(ctx context.Context, magnetURI string, savePath string) error
}

type TorrentHandler struct {
	Store    *torrents.Store
	Adder    TorrentAdder
	SavePath string
}

type createTorrentRequest struct {
	MagnetURI string `json:"magnetUri"`
	Name      string `json:"name"`
}

type torrentResponse struct {
	Torrent torrents.Torrent `json:"torrent"`
}

type torrentsResponse struct {
	Torrents []torrents.Torrent `json:"torrents"`
}

func (h TorrentHandler) ListTorrents(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	records, err := h.Store.ListTorrents(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list torrents")
		return
	}

	writeJSON(w, http.StatusOK, torrentsResponse{Torrents: records})
}

func (h TorrentHandler) CreateTorrent(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var request createTorrentRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	torrent, err := h.Store.CreateTorrent(r.Context(), torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   request.MagnetURI,
		Name:        request.Name,
	})
	if err != nil {
		if errors.Is(err, torrents.ErrInvalidMagnet) {
			writeError(w, http.StatusBadRequest, "invalid magnet URI")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create torrent")
		return
	}

	if h.Adder != nil {
		if err := h.Adder.AddMagnet(r.Context(), torrent.MagnetURI, h.SavePath); err != nil {
			if markErr := h.Store.MarkTorrentError(r.Context(), torrent.ID, err.Error()); markErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to record torrent submission error")
				return
			}
			writeError(w, http.StatusBadGateway, "failed to add torrent to qBittorrent")
			return
		}
	}

	writeJSON(w, http.StatusCreated, torrentResponse{Torrent: torrent})
}
