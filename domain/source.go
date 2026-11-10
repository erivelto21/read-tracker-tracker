package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// SourceStrategy is the way a source's chapter list is parsed.
type SourceStrategy string

const (
	SourceStrategyFeed SourceStrategy = "feed"
	SourceStrategyCSS  SourceStrategy = "css"
	SourceStrategyJSON SourceStrategy = "json"
)

// Source is one crawlable site, stored in the `sites` collection.
//
// The field names and bson tags mirror the crawler's document exactly; the crawler and this API
// share the collection with no compiler between them, so this struct is the contract. A change
// here is a change to the crawler as well.
//
// The document mixes two kinds of state that must never be written together:
//
//   - the declared configuration, which an operator edits through this API;
//   - the runtime health, which only the crawler writes.
//
// Update therefore touches the first group only, so a selector edit cannot erase the evidence
// that the source has been failing.
type Source struct {
	ID            bson.ObjectID  `bson:"_id,omitempty"      json:"id"`
	Host          string         `bson:"host"               json:"host"`
	Name          string         `bson:"name"               json:"name"`
	URLPattern    string         `bson:"url_pattern"        json:"urlPattern"`
	Priority      int            `bson:"priority"           json:"priority"`
	Strategy      SourceStrategy `bson:"strategy"           json:"strategy"`
	Enabled       bool           `bson:"enabled"            json:"enabled"`
	DelayMS       int            `bson:"delay_ms"           json:"delayMs"`
	Profile       string         `bson:"profile"            json:"profile"`
	ListURL       string         `bson:"list_url"           json:"listUrl"`
	FeedURL       string         `bson:"feed_url"           json:"feedUrl"`
	JSONURL       string         `bson:"json_url"           json:"jsonUrl"`
	JSONPath      string         `bson:"json_path"          json:"jsonPath"`
	ItemSelector  string         `bson:"item_selector"      json:"itemSelector"`
	TitleSelector string         `bson:"title_selector"     json:"titleSelector"`
	LinkAttr      string         `bson:"link_attr"          json:"linkAttr"`
	DateSelector  string         `bson:"date_selector"      json:"dateSelector"`
	NumberPattern string         `bson:"number_pattern"     json:"numberPattern"`

	// Provenance. A database edit has no pull request behind it, so the document records who
	// changed it and when.
	CreatedAt time.Time `bson:"created_at"  json:"createdAt"`
	UpdatedAt time.Time `bson:"updated_at"  json:"updatedAt"`
	UpdatedBy string    `bson:"updated_by"  json:"updatedBy"`

	// Runtime health. Written by the crawler, read here, never written by this API.
	LastRunAt           *time.Time `bson:"last_run_at"          json:"lastRunAt,omitempty"`
	LastOKAt            *time.Time `bson:"last_ok_at"           json:"lastOkAt,omitempty"`
	LastError           string     `bson:"last_error"           json:"lastError,omitempty"`
	ConsecutiveFailures int        `bson:"consecutive_failures" json:"consecutiveFailures"`
	Blocked             bool       `bson:"blocked"              json:"blocked"`
	Failing             bool       `bson:"failing"              json:"failing"`
	ItemsFound          int        `bson:"items_found"          json:"itemsFound"`
	ValidatedAt         *time.Time `bson:"validated_at"         json:"validatedAt,omitempty"`
}

// SourcePatch holds the configuration fields that may be changed through a partial update.
// A nil field means "not provided" and will not be written. Runtime health is deliberately absent:
// it is not an operator-editable value.
type SourcePatch struct {
	Host          *string         `json:"host"`
	Name          *string         `json:"name"`
	URLPattern    *string         `json:"urlPattern"`
	Priority      *int            `json:"priority"`
	Strategy      *SourceStrategy `json:"strategy"`
	Enabled       *bool           `json:"enabled"`
	DelayMS       *int            `json:"delayMs"`
	Profile       *string         `json:"profile"`
	ListURL       *string         `json:"listUrl"`
	FeedURL       *string         `json:"feedUrl"`
	JSONURL       *string         `json:"jsonUrl"`
	JSONPath      *string         `json:"jsonPath"`
	ItemSelector  *string         `json:"itemSelector"`
	TitleSelector *string         `json:"titleSelector"`
	LinkAttr      *string         `json:"linkAttr"`
	DateSelector  *string         `json:"dateSelector"`
	NumberPattern *string         `json:"numberPattern"`
}

// Apply returns a copy of s with every provided patch field replaced.
func (p SourcePatch) Apply(s Source) Source {
	if p.Host != nil {
		s.Host = *p.Host
	}
	if p.Name != nil {
		s.Name = *p.Name
	}
	if p.URLPattern != nil {
		s.URLPattern = *p.URLPattern
	}
	if p.Priority != nil {
		s.Priority = *p.Priority
	}
	if p.Strategy != nil {
		s.Strategy = *p.Strategy
	}
	if p.Enabled != nil {
		s.Enabled = *p.Enabled
	}
	if p.DelayMS != nil {
		s.DelayMS = *p.DelayMS
	}
	if p.Profile != nil {
		s.Profile = *p.Profile
	}
	if p.ListURL != nil {
		s.ListURL = *p.ListURL
	}
	if p.FeedURL != nil {
		s.FeedURL = *p.FeedURL
	}
	if p.JSONURL != nil {
		s.JSONURL = *p.JSONURL
	}
	if p.JSONPath != nil {
		s.JSONPath = *p.JSONPath
	}
	if p.ItemSelector != nil {
		s.ItemSelector = *p.ItemSelector
	}
	if p.TitleSelector != nil {
		s.TitleSelector = *p.TitleSelector
	}
	if p.LinkAttr != nil {
		s.LinkAttr = *p.LinkAttr
	}
	if p.DateSelector != nil {
		s.DateSelector = *p.DateSelector
	}
	if p.NumberPattern != nil {
		s.NumberPattern = *p.NumberPattern
	}
	return s
}

// IsEmpty reports whether the patch carries nothing to write.
func (p SourcePatch) IsEmpty() bool {
	return p.Host == nil && p.Name == nil && p.URLPattern == nil && p.Priority == nil &&
		p.Strategy == nil && p.Enabled == nil && p.DelayMS == nil && p.Profile == nil &&
		p.ListURL == nil && p.FeedURL == nil && p.JSONURL == nil && p.JSONPath == nil &&
		p.ItemSelector == nil && p.TitleSelector == nil && p.LinkAttr == nil &&
		p.DateSelector == nil && p.NumberPattern == nil
}

// SourceConfigError names the field that makes a source unusable, so the API can answer with a
// field-level message instead of a bare "invalid".
type SourceConfigError struct {
	Field  string
	Reason string
}

func (e *SourceConfigError) Error() string {
	return fmt.Sprintf("field %q: %s", e.Field, e.Reason)
}

// ValidateSource checks a source's declared configuration against its strategy.
//
// It is the same set of rules the crawler applies before a run, duplicated rather than shared
// because the two are separate modules with no import path between them. Accepting a document the
// crawler will reject is worse than maintaining the rules twice: the source would look configured
// and silently never be crawled.
func ValidateSource(s Source) error {
	if s.Host == "" {
		return &SourceConfigError{Field: "host", Reason: "is required"}
	}
	if s.URLPattern == "" {
		return &SourceConfigError{Field: "urlPattern", Reason: "is required"}
	}
	if _, err := regexp.Compile(s.URLPattern); err != nil {
		return &SourceConfigError{Field: "urlPattern", Reason: "is not a valid regular expression"}
	}
	if s.Profile == "" {
		return &SourceConfigError{Field: "profile", Reason: "is required"}
	}
	if s.DelayMS < 0 {
		return &SourceConfigError{Field: "delayMs", Reason: "must not be negative"}
	}

	switch s.Strategy {
	case SourceStrategyFeed:
		if s.FeedURL == "" {
			return &SourceConfigError{Field: "feedUrl", Reason: "is required for the feed strategy"}
		}

	case SourceStrategyCSS:
		required := []struct{ field, value string }{
			{"listUrl", s.ListURL},
			{"itemSelector", s.ItemSelector},
			{"titleSelector", s.TitleSelector},
			{"linkAttr", s.LinkAttr},
		}
		for _, r := range required {
			if r.value == "" {
				return &SourceConfigError{Field: r.field, Reason: "is required for the css strategy"}
			}
		}

	case SourceStrategyJSON:
		required := []struct{ field, value string }{
			{"jsonUrl", s.JSONURL},
			{"jsonPath", s.JSONPath},
		}
		for _, r := range required {
			if r.value == "" {
				return &SourceConfigError{Field: r.field, Reason: "is required for the json strategy"}
			}
		}

	case "":
		return &SourceConfigError{Field: "strategy", Reason: "is required"}

	default:
		return &SourceConfigError{Field: "strategy", Reason: "must be feed, css or json"}
	}

	return nil
}

var (
	// ErrSourceNotFound reports that a source document does not exist.
	ErrSourceNotFound = errors.New("source not found")
	// ErrSourceExists reports that a source with the same host and URL pattern already exists.
	ErrSourceExists = errors.New("a source with this host and url pattern already exists")
	// ErrEmptyPatch reports an update that carries no fields.
	ErrEmptyPatch = errors.New("at least one field must be provided")
)
