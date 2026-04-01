package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/thesimonho/warden/api"
)

// clipboardDir is the staging directory inside the container where the
// xclip shim reads clipboard images from. Keep in sync with
// container/scripts/shared/install-clipboard-shim.sh.
const clipboardDir = "/tmp/warden-clipboard"

// UploadClipboard stages a file in the container's clipboard directory
// for the xclip shim to serve. Used by the web frontend to enable image
// paste — the browser uploads the image, then sends Ctrl+V to the PTY.
// The agent calls xclip, and the shim returns the staged file.
func (s *Service) UploadClipboard(ctx context.Context, projectID string, content []byte, mimeType string) (*api.ClipboardUploadResponse, error) {
	project, err := s.resolveProject(projectID)
	if err != nil {
		return nil, err
	}

	ext := extensionForMIME(mimeType)
	filename := fmt.Sprintf("paste-%d%s", time.Now().UnixMilli(), ext)

	// Copy to /tmp/ with the subdirectory in the tar path. Docker's
	// CopyToContainer creates intermediate directories when extracting,
	// so this works even if /tmp/warden-clipboard/ doesn't exist yet
	// (e.g. containers built before the clipboard shim was added).
	err = s.docker.CopyFileToContainer(
		ctx,
		project.ContainerID,
		"/tmp",
		"warden-clipboard/"+filename,
		bytes.NewReader(content),
		int64(len(content)),
	)
	if err != nil {
		return nil, fmt.Errorf("staging clipboard file: %w", err)
	}

	return &api.ClipboardUploadResponse{
		Path: clipboardDir + "/" + filename,
	}, nil
}

// extensionForMIME returns a file extension for common image MIME types.
func extensionForMIME(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpeg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	default:
		return ".png"
	}
}
