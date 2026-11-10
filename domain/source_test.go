package domain_test

import (
	"errors"
	"testing"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// validFeedSource is a source that satisfies every rule; each test breaks exactly one thing.
func validFeedSource() domain.Source {
	return domain.Source{
		Host:       "example.com",
		Name:       "Example",
		URLPattern: `^https://example\.com/manga/[^/]+$`,
		Strategy:   domain.SourceStrategyFeed,
		Profile:    "chrome",
		FeedURL:    "https://example.com/feed.xml",
	}
}

func TestValidateSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*domain.Source)
		wantField string // empty means the source is valid
	}{
		{
			name:   "accepts a complete feed source",
			mutate: func(*domain.Source) {},
		},
		{
			name:      "rejects a source without a host",
			mutate:    func(s *domain.Source) { s.Host = "" },
			wantField: "host",
		},
		{
			name:      "rejects a source without a url pattern",
			mutate:    func(s *domain.Source) { s.URLPattern = "" },
			wantField: "urlPattern",
		},
		{
			name:      "rejects a url pattern that does not compile",
			mutate:    func(s *domain.Source) { s.URLPattern = "https://example.com/(" },
			wantField: "urlPattern",
		},
		{
			name:      "rejects a source without a profile",
			mutate:    func(s *domain.Source) { s.Profile = "" },
			wantField: "profile",
		},
		{
			name:      "rejects a negative delay",
			mutate:    func(s *domain.Source) { s.DelayMS = -1 },
			wantField: "delayMs",
		},
		{
			name:      "rejects a source without a strategy",
			mutate:    func(s *domain.Source) { s.Strategy = "" },
			wantField: "strategy",
		},
		{
			name:      "rejects an unknown strategy",
			mutate:    func(s *domain.Source) { s.Strategy = "xpath" },
			wantField: "strategy",
		},
		{
			name:      "rejects a feed source without a feed url",
			mutate:    func(s *domain.Source) { s.FeedURL = "" },
			wantField: "feedUrl",
		},
		{
			name: "accepts a complete css source",
			mutate: func(s *domain.Source) {
				s.Strategy = domain.SourceStrategyCSS
				s.FeedURL = ""
				s.ListURL = "https://example.com/manga"
				s.ItemSelector = "li.chapter"
				s.TitleSelector = "a"
				s.LinkAttr = "href"
			},
		},
		{
			name: "rejects a css source without an item selector",
			mutate: func(s *domain.Source) {
				s.Strategy = domain.SourceStrategyCSS
				s.FeedURL = ""
				s.ListURL = "https://example.com/manga"
				s.TitleSelector = "a"
				s.LinkAttr = "href"
			},
			wantField: "itemSelector",
		},
		{
			name: "rejects a css source without a list url",
			mutate: func(s *domain.Source) {
				s.Strategy = domain.SourceStrategyCSS
				s.FeedURL = ""
				s.ItemSelector = "li.chapter"
				s.TitleSelector = "a"
				s.LinkAttr = "href"
			},
			wantField: "listUrl",
		},
		{
			name: "accepts a complete json source",
			mutate: func(s *domain.Source) {
				s.Strategy = domain.SourceStrategyJSON
				s.FeedURL = ""
				s.JSONURL = "https://example.com/api/chapters"
				s.JSONPath = "$.chapters[*]"
			},
		},
		{
			name: "rejects a json source without a path",
			mutate: func(s *domain.Source) {
				s.Strategy = domain.SourceStrategyJSON
				s.FeedURL = ""
				s.JSONURL = "https://example.com/api/chapters"
			},
			wantField: "jsonPath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := validFeedSource()
			tt.mutate(&source)

			err := domain.ValidateSource(source)

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("expected a valid source, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected an error naming %q, got nil", tt.wantField)
			}

			var configErr *domain.SourceConfigError
			if !errors.As(err, &configErr) {
				t.Fatalf("expected a *SourceConfigError, got %T: %v", err, err)
			}
			if configErr.Field != tt.wantField {
				t.Fatalf("expected field %q, got %q", tt.wantField, configErr.Field)
			}
		})
	}
}

func TestSourcePatchApply(t *testing.T) {
	t.Parallel()

	original := validFeedSource()
	original.Name = "Original"
	original.Priority = 10

	enabled := false
	priority := 42
	name := "Renamed"

	patch := domain.SourcePatch{
		Name:     &name,
		Priority: &priority,
		Enabled:  &enabled,
	}

	merged := patch.Apply(original)

	if merged.Name != "Renamed" {
		t.Errorf("expected the patched name, got %q", merged.Name)
	}
	if merged.Priority != 42 {
		t.Errorf("expected the patched priority, got %d", merged.Priority)
	}
	if merged.Enabled {
		t.Error("expected enabled to be false after the patch")
	}
	// An unpatched field must survive the merge, otherwise a partial update would silently
	// erase the rest of the configuration.
	if merged.Host != original.Host {
		t.Errorf("expected host %q to be preserved, got %q", original.Host, merged.Host)
	}
	if merged.FeedURL != original.FeedURL {
		t.Errorf("expected feed url %q to be preserved, got %q", original.FeedURL, merged.FeedURL)
	}
	// The original value must not be mutated by Apply.
	if original.Name != "Original" {
		t.Errorf("Apply mutated its input: name is now %q", original.Name)
	}
}

func TestSourcePatchIsEmpty(t *testing.T) {
	t.Parallel()

	if !(domain.SourcePatch{}).IsEmpty() {
		t.Error("expected an empty patch to report itself empty")
	}

	enabled := true
	if (domain.SourcePatch{Enabled: &enabled}).IsEmpty() {
		t.Error("expected a patch carrying one field to report itself non-empty")
	}
}
