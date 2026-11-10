package repository

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// MongoChapterRepository implements chapter persistence using MongoDB.
type MongoChapterRepository struct {
	collection *mongo.Collection
}

// NewMongoChapterRepository returns a new MongoChapterRepository.
func NewMongoChapterRepository(col *mongo.Collection) *MongoChapterRepository {
	return &MongoChapterRepository{collection: col}
}

// FindAll returns chapters matching the filter, newest-discovered first.
//
// Pagination is a keyset cursor over (discovered_at, _id): an offset would drift as the crawler
// inserts rows between requests, which is exactly what happens here every night.
func (r *MongoChapterRepository) FindAll(ctx context.Context, filter domain.ChapterFilter) ([]domain.Chapter, error) {
	query := bson.M{}

	if filter.Unread != nil && *filter.Unread {
		query["read_at"] = nil
	}
	if filter.ExcludeImported {
		query["imported"] = false
	}
	if filter.TitleID != nil {
		query["title_id"] = *filter.TitleID
	}
	if filter.Host != nil {
		query["host"] = *filter.Host
	}

	if filter.Cursor != nil && *filter.Cursor != "" {
		after, err := decodeCursor(*filter.Cursor)
		if err != nil {
			return nil, err
		}
		query["$or"] = []bson.M{
			{"discovered_at": bson.M{"$lt": after.discoveredAt}},
			{"discovered_at": after.discoveredAt, "_id": bson.M{"$lt": after.id}},
		}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	findOptions := options.Find().
		SetSort(bson.D{{Key: "discovered_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAll: %w", err)
	}

	chapters := make([]domain.Chapter, 0, limit)
	if err := cursor.All(ctx, &chapters); err != nil {
		return nil, fmt.Errorf("repository.FindAll: %w", err)
	}
	return chapters, nil
}

// FindByID retrieves a chapter by its internal identifier.
func (r *MongoChapterRepository) FindByID(ctx context.Context, id bson.ObjectID) (*domain.Chapter, error) {
	var chapter domain.Chapter
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&chapter)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrChapterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.FindByID: %w", err)
	}
	return &chapter, nil
}

// MarkRead sets the read timestamp on a chapter, preserving an existing one so the operation is
// idempotent. Returns domain.ErrChapterNotFound when no document matches.
func (r *MongoChapterRepository) MarkRead(ctx context.Context, id bson.ObjectID, at time.Time) (*domain.Chapter, error) {
	after := options.After
	var updated domain.Chapter

	// A plain $set stores the array ["$read_at", at] as a literal, corrupting the field. The
	// conditional assignment has to run as an aggregation pipeline.
	pipeline := bson.A{
		bson.M{"$set": bson.M{"read_at": bson.M{"$ifNull": bson.A{"$read_at", at}}}},
	}

	err := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		pipeline,
		options.FindOneAndUpdate().SetReturnDocument(after),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrChapterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.MarkRead: %w", err)
	}
	return &updated, nil
}

// UnreadCounts returns the total unread count and the unread count excluding imported chapters,
// optionally scoped to one title.
func (r *MongoChapterRepository) UnreadCounts(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error) {
	base := bson.M{"read_at": nil}
	if titleID != nil {
		base["title_id"] = *titleID
	}

	total, err := r.collection.CountDocuments(ctx, base)
	if err != nil {
		return domain.UnreadCount{}, fmt.Errorf("repository.UnreadCounts: %w", err)
	}

	excluding := bson.M{"read_at": nil, "imported": false}
	if titleID != nil {
		excluding["title_id"] = *titleID
	}

	withoutImported, err := r.collection.CountDocuments(ctx, excluding)
	if err != nil {
		return domain.UnreadCount{}, fmt.Errorf("repository.UnreadCounts: %w", err)
	}

	return domain.UnreadCount{Total: total, ExcludingImported: withoutImported}, nil
}

// cursor is a decoded pagination position.
type cursor struct {
	discoveredAt time.Time
	id           bson.ObjectID
}

// EncodeCursor builds an opaque cursor for the last chapter of a page.
func EncodeCursor(chapter domain.Chapter) string {
	raw := chapter.DiscoveredAt.UTC().Format(time.RFC3339Nano) + "|" + chapter.ID.Hex()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor parses an opaque cursor. A malformed one is a client error, reported as
// domain.ErrInvalidCursor rather than silently restarting from the first page.
func decodeCursor(raw string) (cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor{}, fmt.Errorf("%w: %v", domain.ErrInvalidCursor, err)
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return cursor{}, fmt.Errorf("%w: malformed position", domain.ErrInvalidCursor)
	}

	discoveredAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return cursor{}, fmt.Errorf("%w: %v", domain.ErrInvalidCursor, err)
	}

	id, err := bson.ObjectIDFromHex(parts[1])
	if err != nil {
		return cursor{}, fmt.Errorf("%w: %v", domain.ErrInvalidCursor, err)
	}

	return cursor{discoveredAt: discoveredAt, id: id}, nil
}
