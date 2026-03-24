package service

import (
	"strings"
	"testing"
)

func TestParseBookmarkHTMLChromeStyle(t *testing.T) {
	input := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
  <DT><H3>Bookmarks Bar</H3>
  <DL><p>
    <DT><A HREF="https://example.com">Example</A>
    <DT><H3>Dev</H3>
    <DL><p>
      <DT><A HREF="https://go.dev">Go</A>
    </DL><p>
  </DL><p>
  <DT><A HREF="https://root.test">Root Link</A>
</DL><p>`

	groups, err := parseBookmarkHTML(strings.NewReader(input), "Imported Bookmarks")
	if err != nil {
		t.Fatalf("parseBookmarkHTML returned error: %v", err)
	}

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if groups[0].Name != "Bookmarks Bar" || len(groups[0].Links) != 1 {
		t.Fatalf("unexpected first group: %#v", groups[0])
	}
	if groups[1].Name != "Bookmarks Bar / Dev" || len(groups[1].Links) != 1 {
		t.Fatalf("unexpected nested group: %#v", groups[1])
	}
	if groups[2].Name != "Imported Bookmarks" || len(groups[2].Links) != 1 {
		t.Fatalf("unexpected root group: %#v", groups[2])
	}
}

func TestParseBookmarkHTMLFirefoxStyle(t *testing.T) {
	input := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks Menu</H1>
<DL><p>
  <DT><H3 PERSONAL_TOOLBAR_FOLDER="true">Bookmarks Toolbar</H3>
  <DL><p>
    <DT><A HREF="https://mozilla.org">Mozilla</A>
  </DL><p>
  <DT><H3>Bookmarks Menu</H3>
  <DL><p>
    <DT><A HREF="https://www.wikipedia.org">Wikipedia</A>
  </DL><p>
</DL><p>`

	groups, err := parseBookmarkHTML(strings.NewReader(input), "Imported Bookmarks")
	if err != nil {
		t.Fatalf("parseBookmarkHTML returned error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "Bookmarks Toolbar" || groups[0].Links[0].URL != "https://mozilla.org" {
		t.Fatalf("unexpected toolbar group: %#v", groups[0])
	}
	if groups[1].Name != "Bookmarks Menu" || groups[1].Links[0].Title != "Wikipedia" {
		t.Fatalf("unexpected menu group: %#v", groups[1])
	}
}

func TestParseBookmarkHTMLRejectsEmptyFile(t *testing.T) {
	_, err := parseBookmarkHTML(strings.NewReader("<html><body><p>empty</p></body></html>"), "Imported Bookmarks")
	if err == nil {
		t.Fatal("expected error for file without bookmarks")
	}
}
