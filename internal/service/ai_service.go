package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"panel/internal/model"

	"golang.org/x/net/html"
)

const (
	aiRequestTimeout = 20 * time.Minute
	aiMetadataLimit  = 1 << 20
)

// AIService provides optional AI-assisted link workflows.
type AIService struct {
	client *http.Client
}

// NewAIService creates an AI service.
func NewAIService() *AIService {
	return &AIService{client: &http.Client{Timeout: aiRequestTimeout}}
}

// LinkEnrichRequest stores fields used to enrich a link.
type LinkEnrichRequest struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	GroupID     string `json:"group_id"`
	Lang        string `json:"lang"`
}

// LinkEnrichResult stores AI-generated link metadata.
type LinkEnrichResult struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	GroupID     string   `json:"group_id"`
	GroupName   string   `json:"group_name"`
	Tags        []string `json:"tags"`
}

// AISearchItem stores a link candidate for AI search.
type AISearchItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	GroupName   string `json:"group_name"`
	URL         string `json:"url"`
}

// AISearchResult stores matching IDs from AI search.
type AISearchResult struct {
	IDs []string `json:"ids"`
}

type pageMetadata struct {
	Title       string
	Description string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Enabled reports whether the config can make AI calls.
func (s *AIService) Enabled(cfg AIConfig) bool {
	if !cfg.Enabled || normalizeAIProvider(cfg.Provider) == "disabled" {
		return false
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return false
	}
	if normalizeAIProvider(cfg.Provider) == "openai" && strings.TrimSpace(cfg.APIKey) == "" {
		return false
	}
	return true
}

// EnrichLink returns AI-generated title, description, and group suggestion.
func (s *AIService) EnrichLink(ctx context.Context, cfg AIConfig, input LinkEnrichRequest, groups []model.NavGroup) (LinkEnrichResult, error) {
	if !s.Enabled(cfg) {
		return LinkEnrichResult{}, errors.New("AI is not configured")
	}

	meta := fetchPageMetadata(ctx, input.URL)
	groupLines := make([]string, 0, len(groups))
	for _, group := range groups {
		groupLines = append(groupLines, fmt.Sprintf("- %s: %s", group.ID, group.Name))
	}

	lang := strings.TrimSpace(input.Lang)
	if lang == "" {
		lang = "zh"
	}

	prompt := fmt.Sprintf(`Return compact JSON only. Enrich this bookmark for a personal dashboard.
Language: %s
URL: %s
Existing title: %s
Existing description: %s
Fetched page title: %s
Fetched page description: %s
Available groups, formatted as "id: name":
%s

Rules:
- Keep original brand/site names when useful.
- Description should be concise, one sentence, and in the requested language.
- Choose group_id from the available group ids when there is a clear match; otherwise use the existing group_id.
- tags should contain 1 to 4 short labels.

JSON shape:
{"title":"...","description":"...","group_id":"...","group_name":"...","tags":["..."]}`,
		lang,
		strings.TrimSpace(input.URL),
		strings.TrimSpace(input.Title),
		strings.TrimSpace(input.Description),
		meta.Title,
		meta.Description,
		strings.Join(groupLines, "\n"),
	)

	content, err := s.chat(ctx, cfg, prompt)
	if err != nil {
		return LinkEnrichResult{}, err
	}

	var result LinkEnrichResult
	if err := decodeJSONContent(content, &result); err != nil {
		return LinkEnrichResult{}, err
	}
	result.Title = strings.TrimSpace(firstNonEmpty(result.Title, input.Title, meta.Title))
	result.Description = strings.TrimSpace(firstNonEmpty(result.Description, input.Description, meta.Description))
	result.GroupID = strings.TrimSpace(firstNonEmpty(result.GroupID, input.GroupID))
	if !groupIDExists(groups, result.GroupID) {
		result.GroupID = strings.TrimSpace(input.GroupID)
	}
	return result, nil
}

// SearchLinks asks AI to return matching link IDs for a natural-language query.
func (s *AIService) SearchLinks(ctx context.Context, cfg AIConfig, query string, items []AISearchItem, lang string) (AISearchResult, error) {
	if !s.Enabled(cfg) {
		return AISearchResult{}, errors.New("AI is not configured")
	}
	if strings.TrimSpace(query) == "" {
		return AISearchResult{}, errors.New("query is required")
	}
	if len(items) > 300 {
		items = items[:300]
	}
	payload, _ := json.Marshal(items)
	prompt := fmt.Sprintf(`Return compact JSON only. Search these bookmarks using the user's natural-language query.
Language: %s
Query: %s
Bookmarks JSON: %s

Return the best matching bookmark IDs in relevance order. Include at most 30 IDs.
JSON shape:
{"ids":["link-id"]}`, strings.TrimSpace(lang), strings.TrimSpace(query), string(payload))

	content, err := s.chat(ctx, cfg, prompt)
	if err != nil {
		return AISearchResult{}, err
	}

	var result AISearchResult
	if err := decodeJSONContent(content, &result); err != nil {
		return AISearchResult{}, err
	}
	return result, nil
}

// TestConnection verifies that the configured provider can answer a minimal chat request.
func (s *AIService) TestConnection(ctx context.Context, cfg AIConfig) error {
	if !s.Enabled(cfg) {
		return errors.New("AI is not configured")
	}
	content, err := s.chat(ctx, cfg, `Return compact JSON only: {"ok":true}`)
	if err != nil {
		return err
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := decodeJSONContent(content, &result); err != nil {
		return fmt.Errorf("AI response was not valid JSON: %w", err)
	}
	if !result.OK {
		return errors.New("AI test response did not return ok=true")
	}
	return nil
}

func (s *AIService) chat(ctx context.Context, cfg AIConfig, prompt string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return "", errors.New("AI base URL is required")
	}
	endpoint := baseURL + "/chat/completions"
	requestBody := chatCompletionRequest{
		Model: strings.TrimSpace(cfg.Model),
		Messages: []chatMessage{
			{Role: "system", Content: "You are a precise assistant for bookmark organization. Always return valid JSON and no markdown."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(cfg.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI request failed: %s", strings.TrimSpace(string(body)))
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("AI response has no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

func fetchPageMetadata(ctx context.Context, rawURL string) pageMetadata {
	parsed, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return pageMetadata{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return pageMetadata{}
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return pageMetadata{}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return pageMetadata{}
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, aiMetadataLimit))
	if err != nil {
		return pageMetadata{}
	}
	var meta pageMetadata
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "title" && node.FirstChild != nil && meta.Title == "" {
			meta.Title = strings.TrimSpace(node.FirstChild.Data)
		}
		if node.Type == html.ElementNode && node.Data == "meta" {
			var name, property, content string
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "name":
					name = strings.ToLower(strings.TrimSpace(attr.Val))
				case "property":
					property = strings.ToLower(strings.TrimSpace(attr.Val))
				case "content":
					content = strings.TrimSpace(attr.Val)
				}
			}
			if meta.Description == "" && content != "" && (name == "description" || property == "og:description") {
				meta.Description = content
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return meta
}

func decodeJSONContent(content string, target any) error {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			content = content[start : end+1]
		}
	}
	return json.Unmarshal([]byte(content), target)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func groupIDExists(groups []model.NavGroup, groupID string) bool {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return false
	}
	for _, group := range groups {
		if group.ID == groupID {
			return true
		}
	}
	return false
}
