package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"panel/pkg/runtimepath"

	"github.com/chai2010/webp"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/net/html"
)

const (
	maxIconWidth       = 128
	maxIconHeight      = 128
	iconWebPQuality    = 84
	maxFetchedIconSize = 5 << 20
)

// FetchWebsiteIcon tries to fetch, optimize, and store a website favicon locally.
func FetchWebsiteIcon(ctx context.Context, rawURL, uploadDir string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil
	}

	pageURL, err := neturl.Parse(rawURL)
	if err != nil || pageURL.Scheme == "" || pageURL.Host == "" {
		return "", nil
	}

	client := &http.Client{Timeout: 8 * time.Second}
	candidates := collectIconCandidates(ctx, client, pageURL)
	candidates = append(candidates, pageURL.Scheme+"://"+pageURL.Host+"/favicon.ico")

	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		publicPath, err := fetchAndStoreIconCandidate(ctx, client, candidate, uploadDir)
		if err == nil && publicPath != "" {
			return publicPath, nil
		}
	}

	return "", nil
}

func collectIconCandidates(ctx context.Context, client *http.Client, pageURL *neturl.URL) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/html") {
		return nil
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}

	var candidates []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "link" {
			var relValue, hrefValue string
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "rel":
					relValue = strings.ToLower(strings.TrimSpace(attr.Val))
				case "href":
					hrefValue = strings.TrimSpace(attr.Val)
				}
			}
			if hrefValue != "" && (strings.Contains(relValue, "icon") || strings.Contains(relValue, "apple-touch-icon")) {
				if resolved := resolveIconURL(pageURL, hrefValue); resolved != "" {
					candidates = append(candidates, resolved)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return candidates
}

func resolveIconURL(base *neturl.URL, href string) string {
	parsed, err := neturl.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func fetchAndStoreIconCandidate(ctx context.Context, client *http.Client, candidate, uploadDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("icon request returned %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchedIconSize))
	if err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("empty icon payload")
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	switch {
	case strings.Contains(contentType, "image/x-icon"), strings.Contains(contentType, "image/vnd.microsoft.icon"):
		return saveRawIcon(uploadDir, payload, ".ico")
	case strings.Contains(contentType, "image/"):
		imageData, _, err := image.Decode(bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		optimized := resizeImage(imageData, maxIconWidth, maxIconHeight)
		return saveOptimizedIcon(uploadDir, optimized)
	default:
		ext := strings.ToLower(path.Ext(candidate))
		if ext == ".ico" {
			return saveRawIcon(uploadDir, payload, ".ico")
		}
		imageData, _, err := image.Decode(bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		optimized := resizeImage(imageData, maxIconWidth, maxIconHeight)
		return saveOptimizedIcon(uploadDir, optimized)
	}
}

func saveOptimizedIcon(uploadDir string, img image.Image) (string, error) {
	iconDir := runtimepath.IconsDir(uploadDir)
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return "", fmt.Errorf("create icon dir: %w", err)
	}

	filename := fmt.Sprintf("icon-%d.webp", time.Now().UnixNano())
	fullPath := filepath.Join(iconDir, filename)
	output, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create icon file: %w", err)
	}
	defer output.Close()

	if err := webp.Encode(output, img, &webp.Options{Lossless: false, Quality: iconWebPQuality}); err != nil {
		return "", fmt.Errorf("save icon file: %w", err)
	}

	return runtimepath.PublicUploadPath(filepath.Join(runtimepath.IconsDirName, filename)), nil
}

func saveRawIcon(uploadDir string, payload []byte, ext string) (string, error) {
	iconDir := runtimepath.IconsDir(uploadDir)
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return "", fmt.Errorf("create icon dir: %w", err)
	}

	filename := fmt.Sprintf("icon-%d%s", time.Now().UnixNano(), ext)
	fullPath := filepath.Join(iconDir, filename)
	if err := os.WriteFile(fullPath, payload, 0o644); err != nil {
		return "", fmt.Errorf("save icon file: %w", err)
	}

	return runtimepath.PublicUploadPath(filepath.Join(runtimepath.IconsDirName, filename)), nil
}

func resizeImage(src image.Image, maxWidth, maxHeight int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}

	scale := minIconFloat(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	if scale >= 1 {
		return src
	}

	targetWidth := maxIconInt(1, int(float64(width)*scale))
	targetHeight := maxIconInt(1, int(float64(height)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}

func minIconFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxIconInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
