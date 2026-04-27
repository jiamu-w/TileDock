package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"panel/pkg/runtimepath"

	"github.com/chai2010/webp"
	xdraw "golang.org/x/image/draw"
)

const (
	maxFetchedThumbnailSize = 8 << 20
	thumbnailMaxWidth       = 640
	thumbnailMaxHeight      = 420
	thumbnailWebPQuality    = 78
)

// FetchWebsiteThumbnail downloads a website screenshot from a third-party thumbnail service and stores it locally.
func FetchWebsiteThumbnail(ctx context.Context, rawURL, uploadDir string) (string, error) {
	pageURL, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil || pageURL.Scheme == "" || pageURL.Host == "" {
		return "", nil
	}

	client := &http.Client{Timeout: 12 * time.Second}
	candidates := []string{
		"https://s.wordpress.com/mshots/v1/" + neturl.QueryEscape(pageURL.String()) + "?w=640",
		"https://image.thum.io/get/width/640/crop/420/noanimate/" + pageURL.String(),
	}

	for _, candidate := range candidates {
		publicPath, err := fetchAndStoreThumbnailCandidate(ctx, client, candidate, uploadDir)
		if err == nil && publicPath != "" {
			return publicPath, nil
		}
	}

	return "", nil
}

func fetchAndStoreThumbnailCandidate(ctx context.Context, client *http.Client, candidate, uploadDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TileDock/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("thumbnail request returned %d", resp.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !strings.Contains(contentType, "image/") || strings.Contains(contentType, "svg") {
		return "", fmt.Errorf("thumbnail content type is not supported: %s", contentType)
	}

	payload, err := readLimited(resp.Body, maxFetchedThumbnailSize)
	if err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("empty thumbnail payload")
	}

	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	return saveOptimizedThumbnail(uploadDir, resizeThumbnail(img, thumbnailMaxWidth, thumbnailMaxHeight))
}

func saveOptimizedThumbnail(uploadDir string, img image.Image) (string, error) {
	thumbnailDir := runtimepath.ThumbnailsDir(uploadDir)
	if err := os.MkdirAll(thumbnailDir, 0o755); err != nil {
		return "", fmt.Errorf("create thumbnail dir: %w", err)
	}

	filename := fmt.Sprintf("thumbnail-%d.webp", time.Now().UnixNano())
	fullPath := filepath.Join(thumbnailDir, filename)
	output, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create thumbnail file: %w", err)
	}
	defer output.Close()

	if err := webp.Encode(output, img, &webp.Options{Lossless: false, Quality: thumbnailWebPQuality}); err != nil {
		return "", fmt.Errorf("save thumbnail file: %w", err)
	}

	return runtimepath.PublicUploadPath(filepath.Join(runtimepath.ThumbnailsDirName, filename)), nil
}

func resizeThumbnail(src image.Image, maxWidth, maxHeight int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}

	scale := minIconFloat(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	if scale > 1 {
		scale = 1
	}

	targetWidth := maxIconInt(1, int(float64(width)*scale))
	targetHeight := maxIconInt(1, int(float64(height)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
