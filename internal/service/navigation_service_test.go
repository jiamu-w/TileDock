package service

import "testing"

func TestLinkInputValidate(t *testing.T) {
	input := LinkInput{
		GroupID: "group-1",
		Title:   "Search",
		URL:     "https://google.com",
	}

	if err := input.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
