package domain

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Chapter is one released chapter of a tracked title, written by the crawler and read here.
//
// The field names and types mirror the crawler's document exactly; the two repositories share
// this collection and have no compiler between them, so this struct is the contract. A change
// here is a change to the crawler as well.
type Chapter struct {
	ID           bson.ObjectID `bson:"_id,omitempty"    json:"-"`
	TitleID      bson.ObjectID `bson:"title_id"         json:"titleId"`
	Host         string        `bson:"host"             json:"host"`
	Key          string        `bson:"dedup_key"        json:"-"`
	Number       *float64      `bson:"number"           json:"number"`
	NumberRaw    string        `bson:"number_raw"       json:"numberRaw"`
	Name         string        `bson:"name"             json:"name"`
	URL          string        `bson:"url"              json:"url"`
	PublishedAt  *time.Time    `bson:"published_at"     json:"publishedAt,omitempty"`
	DiscoveredAt time.Time     `bson:"discovered_at"    json:"discoveredAt"`
	ReadAt       *time.Time    `bson:"read_at"          json:"readAt,omitempty"`
	Imported     bool          `bson:"imported"         json:"imported"`
}

// ChapterView is a chapter as returned to clients, carrying the owning title's display name so
// the new-chapters screen does not have to join on the client.
type ChapterView struct {
	ID           string     `json:"id"`
	TitleID      string     `json:"titleId"`
	TitleName    string     `json:"titleName"`
	Host         string     `json:"host"`
	Number       *float64   `json:"number"`
	NumberRaw    string     `json:"numberRaw"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	DiscoveredAt time.Time  `json:"discoveredAt"`
	ReadAt       *time.Time `json:"readAt,omitempty"`
	Imported     bool       `json:"imported"`
}

// ChapterFilter holds the optional filters for listing chapters.
type ChapterFilter struct {
	Unread          *bool
	ExcludeImported bool
	TitleID         *bson.ObjectID
	Host            *string
	Cursor          *string
	Limit           int
}

// UnreadCount reports how many chapters are unread, with and without the chapters imported on a
// title's first crawl.
type UnreadCount struct {
	Total             int64 `json:"total"`
	ExcludingImported int64 `json:"excludingImported"`
}

var (
	// ErrChapterNotFound reports that a chapter does not exist.
	ErrChapterNotFound = errors.New("chapter not found")
	// ErrInvalidCursor reports a pagination cursor that cannot be decoded.
	ErrInvalidCursor = errors.New("invalid cursor")
)
