package domain

import (
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TitleType represents the type of a reading title.
type TitleType string

const (
	Book    TitleType = "book"
	Manga   TitleType = "manga"
	Manhua  TitleType = "manhua"
	Novel   TitleType = "novel"
	Article TitleType = "article"
)

// Title represents a reading entry tracked by the user.
// ID is the internal MongoDB ObjectID, never exposed in responses.
// ExternalID is the public UUID used in API responses and future lookups.
type Title struct {
	ID          bson.ObjectID `bson:"_id,omitempty"      json:"-"`
	ExternalID  string        `bson:"external_id"        json:"id"`
	Name        string        `bson:"name"               json:"name"`
	Type        TitleType     `bson:"type"               json:"type"`
	Chapter     *int          `bson:"chapter,omitempty"  json:"chapter,omitempty"`
	Page        *int          `bson:"page,omitempty"     json:"page,omitempty"`
	Link        *string       `bson:"link,omitempty"     json:"link,omitempty"`
	Observation *string       `bson:"observation,omitempty" json:"observation,omitempty"`
}

// TitleFilter holds optional filters for the List operation.
type TitleFilter struct {
	Type *TitleType
	Name *string
}

var (
	ErrAlreadyExists = errors.New("title already exists")
	ErrNotFound      = errors.New("title not found")
)
