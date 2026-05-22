package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/binoy638/streamize-api/apps/api/internal/catalog"
	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/metadata"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

type LibraryHandler struct {
	Store        *catalog.Store
	TorrentStore *torrents.Store
	JobStore     *jobs.Store
	Metadata     metadata.Resolver
}

type libraryResponse struct {
	Items []catalog.LibraryItem `json:"items"`
}

type metadataSearchResponse struct {
	Results []metadata.Match `json:"results"`
}

type metadataRefreshResponse struct {
	Job jobs.Job `json:"job"`
}

type manualMetadataMatchRequest struct {
	MediaType       string `json:"mediaType"`
	Provider        string `json:"provider"`
	ProviderID      string `json:"providerId"`
	Title           string `json:"title"`
	OriginalTitle   string `json:"originalTitle"`
	Overview        string `json:"overview"`
	ReleaseYear     int    `json:"releaseYear"`
	PosterURL       string `json:"posterUrl"`
	BackdropURL     string `json:"backdropUrl"`
	SeasonNumber    int    `json:"seasonNumber"`
	EpisodeNumber   int    `json:"episodeNumber"`
	AbsoluteNumber  int    `json:"absoluteNumber"`
	EpisodeTitle    string `json:"episodeTitle"`
	EpisodeOverview string `json:"episodeOverview"`
	AirDate         string `json:"airDate"`
	StillURL        string `json:"stillUrl"`
}

func (h LibraryHandler) ListLibrary(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	items, err := h.Store.ListLibrary(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list library")
		return
	}
	writeJSON(w, http.StatusOK, libraryResponse{Items: items})
}

func (h LibraryHandler) RefreshFileMetadata(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	file, err := h.TorrentStore.FindTorrentFileByIDForOwner(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load file")
		return
	}
	job, _, err := h.JobStore.CreateMetadataIdentifyJobIfMissing(r.Context(), file.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue metadata refresh")
		return
	}
	if isRetryableJob(job.Status) {
		job, err = h.JobStore.Retry(r.Context(), job.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to retry metadata refresh")
			return
		}
	}
	writeJSON(w, http.StatusAccepted, metadataRefreshResponse{Job: job})
}

func (h LibraryHandler) SearchMetadata(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	match, err := h.Metadata.Resolve(r.Context(), query)
	if err != nil {
		if errors.Is(err, metadata.ErrNoMatch) {
			writeJSON(w, http.StatusOK, metadataSearchResponse{Results: nil})
			return
		}
		if errors.Is(err, metadata.ErrProviderNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "metadata provider is not configured")
			return
		}
		writeError(w, http.StatusBadGateway, "metadata lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, metadataSearchResponse{Results: []metadata.Match{match}})
}

func (h LibraryHandler) ManualMatchFile(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	file, err := h.TorrentStore.FindTorrentFileByIDForOwner(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load file")
		return
	}
	var request manualMetadataMatchRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	item, err := h.Store.UpsertItem(r.Context(), catalog.Item{
		OwnerUserID:    user.ID,
		MediaType:      request.MediaType,
		Provider:       request.Provider,
		ProviderID:     request.ProviderID,
		Title:          request.Title,
		OriginalTitle:  request.OriginalTitle,
		Overview:       request.Overview,
		ReleaseYear:    request.ReleaseYear,
		PosterURL:      request.PosterURL,
		BackdropURL:    request.BackdropURL,
		MetadataStatus: catalog.MetadataStatusManual,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save catalog item")
		return
	}

	var episodeID string
	if request.SeasonNumber > 0 || request.EpisodeNumber > 0 || request.AbsoluteNumber > 0 {
		episode, err := h.Store.UpsertEpisode(r.Context(), catalog.Episode{
			CatalogItemID:  item.ID,
			Provider:       request.Provider,
			ProviderID:     request.ProviderID,
			SeasonNumber:   request.SeasonNumber,
			EpisodeNumber:  request.EpisodeNumber,
			AbsoluteNumber: request.AbsoluteNumber,
			Title:          request.EpisodeTitle,
			Overview:       request.EpisodeOverview,
			AirDate:        request.AirDate,
			StillURL:       request.StillURL,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save catalog episode")
			return
		}
		episodeID = episode.ID
	}

	if err := h.Store.LinkFile(r.Context(), catalog.FileLinkParams{
		TorrentFileID:    file.ID,
		CatalogItemID:    item.ID,
		CatalogEpisodeID: episodeID,
		Status:           catalog.MetadataStatusManual,
		Confidence:       1,
		Provider:         request.Provider,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link file metadata")
		return
	}
	writeJSON(w, http.StatusOK, item)
}
