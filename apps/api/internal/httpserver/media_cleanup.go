package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

func (h TorrentHandler) cleanupTorrentGeneratedFiles(ctx context.Context, files []torrents.TorrentFile) error {
	for _, file := range files {
		if err := h.cleanupGeneratedFileAssets(ctx, file); err != nil {
			return err
		}
	}

	return nil
}

func (h TorrentHandler) cleanupTorrentOriginalFiles(ctx context.Context, files []torrents.TorrentFile) error {
	for _, file := range files {
		originalPath := strings.TrimSpace(file.OriginalPath)
		if originalPath == "" {
			continue
		}

		cleaned, ok := safePathInRoot(h.SavePath, originalPath)
		if !ok {
			return fmt.Errorf("unsafe original media path %q", originalPath)
		}
		if err := os.Remove(cleaned); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove original media path %q: %w", cleaned, err)
		}
		h.removeEmptyOriginalParents(cleaned)
		h.debug(ctx, "original media path removed",
			slog.String("torrent_file_id", file.ID),
			slog.String("path", cleaned),
		)
	}

	return nil
}

func (h TorrentHandler) removeEmptyOriginalParents(filePath string) {
	root, err := filepath.Abs(strings.TrimSpace(h.SavePath))
	if err != nil || root == "" {
		return
	}

	dir := filepath.Dir(filePath)
	for {
		relative, err := filepath.Rel(root, dir)
		if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func (h TorrentHandler) cleanupGeneratedFileAssets(ctx context.Context, file torrents.TorrentFile) error {
	paths := make(map[string]generatedMediaPath)
	addGeneratedPath(paths, h.HLSDir, filepath.Join(h.HLSDir, file.ID))
	addGeneratedPath(paths, h.SubtitlesDir, filepath.Join(h.SubtitlesDir, file.ID))
	addGeneratedPath(paths, h.ThumbnailsDir, filepath.Join(h.ThumbnailsDir, file.ID))

	if strings.TrimSpace(file.HLSPath) != "" {
		addGeneratedPath(paths, h.HLSDir, filepath.Dir(file.HLSPath))
	}
	if strings.TrimSpace(file.ThumbnailSheetPath) != "" {
		addGeneratedPath(paths, h.ThumbnailsDir, filepath.Dir(file.ThumbnailSheetPath))
	}
	if strings.TrimSpace(file.ThumbnailVTTPath) != "" {
		addGeneratedPath(paths, h.ThumbnailsDir, filepath.Dir(file.ThumbnailVTTPath))
	}

	for _, item := range paths {
		cleaned, ok := safePathInRoot(item.root, item.path)
		if !ok {
			return fmt.Errorf("unsafe generated media path %q", item.path)
		}
		if err := os.RemoveAll(cleaned); err != nil {
			return fmt.Errorf("remove generated media path %q: %w", cleaned, err)
		}
		h.debug(ctx, "generated media path removed",
			slog.String("torrent_file_id", file.ID),
			slog.String("path", cleaned),
		)
	}

	return nil
}

type generatedMediaPath struct {
	root string
	path string
}

func addGeneratedPath(paths map[string]generatedMediaPath, root string, path string) {
	root = strings.TrimSpace(root)
	path = strings.TrimSpace(path)
	if root == "" || path == "" {
		return
	}

	paths[root+"\x00"+path] = generatedMediaPath{root: root, path: path}
}
