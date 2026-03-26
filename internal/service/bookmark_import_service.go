package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// BookmarkImportService imports browser bookmark exports.
type BookmarkImportService struct {
	nav       *NavigationService
	uploadDir string
}

// BookmarkImportResult stores bookmark import counters.
type BookmarkImportResult struct {
	GroupCount int
	LinkCount  int
}

type bookmarkGroup struct {
	Name  string
	Links []bookmarkLink
}

type bookmarkLink struct {
	Title string
	URL   string
}

// NewBookmarkImportService creates a bookmark import service.
func NewBookmarkImportService(nav *NavigationService, uploadDir string) *BookmarkImportService {
	return &BookmarkImportService{nav: nav, uploadDir: uploadDir}
}

// ImportHTML imports a Chrome/Firefox bookmark export file.
func (s *BookmarkImportService) ImportHTML(ctx context.Context, reader io.Reader, defaultGroupName string) (*BookmarkImportResult, error) {
	groups, err := parseBookmarkHTML(reader, defaultGroupName)
	if err != nil {
		return nil, err
	}

	result := &BookmarkImportResult{}
	for _, group := range groups {
		groupID, err := s.nav.CreateGroupWithID(ctx, group.Name)
		if err != nil {
			return nil, fmt.Errorf("create group %q: %w", group.Name, err)
		}
		result.GroupCount++

		for _, link := range group.Links {
			iconPath, _ := FetchWebsiteIcon(ctx, link.URL, s.uploadDir)
			err := s.nav.CreateLink(ctx, LinkInput{
				GroupID:   groupID,
				Title:     link.Title,
				URL:       link.URL,
				Icon:      iconPath,
				OpenInNew: true,
			})
			if err != nil {
				return nil, fmt.Errorf("create link %q: %w", link.Title, err)
			}
			result.LinkCount++
		}
	}

	return result, nil
}

func parseBookmarkHTML(reader io.Reader, defaultGroupName string) ([]bookmarkGroup, error) {
	doc, err := html.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parse bookmark html: %w", err)
	}

	rootDL := findFirstElement(doc, "dl")
	if rootDL == nil {
		return nil, errors.New("bookmark file format is invalid")
	}

	orderedGroups := make([]string, 0, 8)
	groupMap := make(map[string][]bookmarkLink)
	collectBookmarks(rootDL, nil, strings.TrimSpace(defaultGroupName), groupMap, &orderedGroups)
	if len(orderedGroups) == 0 {
		return nil, errors.New("no bookmarks found in file")
	}

	groups := make([]bookmarkGroup, 0, len(orderedGroups))
	for _, name := range orderedGroups {
		links := groupMap[name]
		if len(links) == 0 {
			continue
		}
		groups = append(groups, bookmarkGroup{
			Name:  name,
			Links: links,
		})
	}
	if len(groups) == 0 {
		return nil, errors.New("no bookmarks found in file")
	}
	return groups, nil
}

func collectBookmarks(node *html.Node, path []string, defaultGroupName string, groups map[string][]bookmarkLink, orderedGroups *[]string) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		switch strings.ToLower(child.Data) {
		case "dt":
			entry := firstElementChild(child)
			if entry == nil {
				continue
			}

			switch strings.ToLower(entry.Data) {
			case "a":
				targetURL := strings.TrimSpace(htmlAttr(entry, "href"))
				if targetURL == "" {
					continue
				}
				title := cleanBookmarkText(nodeText(entry))
				if title == "" {
					title = targetURL
				}

				groupName := bookmarkGroupName(path, defaultGroupName)
				if _, ok := groups[groupName]; !ok {
					*orderedGroups = append(*orderedGroups, groupName)
				}
				groups[groupName] = append(groups[groupName], bookmarkLink{
					Title: title,
					URL:   targetURL,
				})

			case "h3":
				folderName := cleanBookmarkText(nodeText(entry))
				if folderName == "" {
					continue
				}

				nestedDL := findFirstElement(child, "dl")
				if nestedDL == nil {
					nestedDL = nextElementSibling(child)
				}
				if nestedDL != nil && strings.EqualFold(nestedDL.Data, "dl") {
					collectBookmarks(nestedDL, append(path, folderName), defaultGroupName, groups, orderedGroups)
				}
			}

		case "dl":
			collectBookmarks(child, path, defaultGroupName, groups, orderedGroups)
		}
	}
}

func findFirstElement(node *html.Node, tag string) *html.Node {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, tag) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func firstElementChild(node *html.Node) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			return child
		}
	}
	return nil
}

func nextElementSibling(node *html.Node) *html.Node {
	for sibling := node.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == html.ElementNode {
			return sibling
		}
	}
	return nil
}

func htmlAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func cleanBookmarkText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func bookmarkGroupName(path []string, defaultGroupName string) string {
	clean := make([]string, 0, len(path))
	for _, item := range path {
		item = cleanBookmarkText(item)
		if item != "" {
			clean = append(clean, item)
		}
	}
	if len(clean) == 0 {
		if strings.TrimSpace(defaultGroupName) != "" {
			return strings.TrimSpace(defaultGroupName)
		}
		return "Imported Bookmarks"
	}
	return strings.Join(clean, " / ")
}
