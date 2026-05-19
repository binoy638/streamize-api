package httpserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

type JobHandler struct {
	Store        *jobs.Store
	TorrentStore *torrents.Store
}

type jobsResponse struct {
	Jobs []jobs.JobRecord `json:"jobs"`
}

type jobResponse struct {
	Job jobs.JobRecord `json:"job"`
}

func (h JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	records, err := h.Store.ListJobsForOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	writeJSON(w, http.StatusOK, jobsResponse{Jobs: records})
}

func (h JobHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	record, err := h.Store.FindJobByIDForOwner(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load job")
		return
	}
	if !isRetryableJob(record.Status) {
		writeError(w, http.StatusConflict, "job is not retryable")
		return
	}
	if h.TorrentStore != nil && record.Type == jobs.TypeHLSTranscode && record.TorrentFileID != "" {
		if err := h.TorrentStore.ResetTorrentFileForTranscode(r.Context(), record.TorrentFileID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to reset file for retry")
			return
		}
	}

	if _, err := h.Store.Retry(r.Context(), record.ID); err != nil {
		if errors.Is(err, jobs.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "job is not retryable")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to retry job")
		return
	}
	updated, err := h.Store.FindJobByIDForOwner(r.Context(), record.ID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload job")
		return
	}

	writeJSON(w, http.StatusOK, jobResponse{Job: updated})
}

func (h JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	record, err := h.Store.FindJobByIDForOwner(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load job")
		return
	}
	if !isCancelableJob(record.Status) {
		writeError(w, http.StatusConflict, "job is not cancelable")
		return
	}

	if _, err := h.Store.Cancel(r.Context(), record.ID); err != nil {
		if errors.Is(err, jobs.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "job is not cancelable")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to cancel job")
		return
	}
	updated, err := h.Store.FindJobByIDForOwner(r.Context(), record.ID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload job")
		return
	}

	writeJSON(w, http.StatusOK, jobResponse{Job: updated})
}

func isRetryableJob(status string) bool {
	return status == jobs.StatusFailed || status == jobs.StatusCanceled || status == jobs.StatusSucceeded
}

func isCancelableJob(status string) bool {
	return status == jobs.StatusQueued || status == jobs.StatusFailed
}
