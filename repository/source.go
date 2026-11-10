package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// MongoSourceRepository implements source persistence using the crawler's `sites` collection.
//
// The collection is shared with the crawler, which reads it at the start of every run. Writes
// here therefore take effect on the next run, not immediately.
type MongoSourceRepository struct {
	collection *mongo.Collection
}

// NewMongoSourceRepository returns a new MongoSourceRepository.
func NewMongoSourceRepository(col *mongo.Collection) *MongoSourceRepository {
	return &MongoSourceRepository{collection: col}
}

// FindAll returns every source, worst health first so a failing source is the first thing the
// reader sees, then by priority.
func (r *MongoSourceRepository) FindAll(ctx context.Context) ([]domain.Source, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "failing", Value: -1},
		{Key: "blocked", Value: -1},
		{Key: "priority", Value: -1},
		{Key: "host", Value: 1},
	})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAll: %w", err)
	}

	sources := make([]domain.Source, 0)
	if err := cursor.All(ctx, &sources); err != nil {
		return nil, fmt.Errorf("repository.FindAll: %w", err)
	}
	return sources, nil
}

// FindByID retrieves a source by its identifier. Returns domain.ErrSourceNotFound if it does not
// exist, including when the identifier is not a valid ObjectID.
func (r *MongoSourceRepository) FindByID(ctx context.Context, id bson.ObjectID) (*domain.Source, error) {
	var source domain.Source
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&source)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrSourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.FindByID: %w", err)
	}
	return &source, nil
}

// FindByHostAndPattern reports whether a source already claims this host and URL pattern.
// excludeID is skipped when non-nil, so an update can check for conflicts without matching itself.
func (r *MongoSourceRepository) FindByHostAndPattern(
	ctx context.Context, host, pattern string, excludeID *bson.ObjectID,
) (*domain.Source, error) {
	query := bson.M{"host": host, "url_pattern": pattern}
	if excludeID != nil {
		query["_id"] = bson.M{"$ne": *excludeID}
	}

	var source domain.Source
	err := r.collection.FindOne(ctx, query).Decode(&source)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrSourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.FindByHostAndPattern: %w", err)
	}
	return &source, nil
}

// Insert persists a new source. The caller supplies CreatedAt and UpdatedAt.
func (r *MongoSourceRepository) Insert(ctx context.Context, source *domain.Source) (*domain.Source, error) {
	result, err := r.collection.InsertOne(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("repository.Insert: %w", err)
	}

	id, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return nil, fmt.Errorf("repository.Insert: unexpected inserted id type %T", result.InsertedID)
	}
	source.ID = id
	return source, nil
}

// Update applies a partial configuration update and returns the document after the write.
//
// Only the declared configuration is written. The runtime health fields are left untouched, so
// fixing a selector does not clear the record that the source has been failing — that record is
// the only evidence the fix was needed.
func (r *MongoSourceRepository) Update(
	ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, at time.Time, by string,
) (*domain.Source, error) {
	set := bson.M{
		"updated_at": at,
		"updated_by": by,
	}

	if patch.Host != nil {
		set["host"] = *patch.Host
	}
	if patch.Name != nil {
		set["name"] = *patch.Name
	}
	if patch.URLPattern != nil {
		set["url_pattern"] = *patch.URLPattern
	}
	if patch.Priority != nil {
		set["priority"] = *patch.Priority
	}
	if patch.Strategy != nil {
		set["strategy"] = *patch.Strategy
	}
	if patch.Enabled != nil {
		set["enabled"] = *patch.Enabled
	}
	if patch.DelayMS != nil {
		set["delay_ms"] = *patch.DelayMS
	}
	if patch.Profile != nil {
		set["profile"] = *patch.Profile
	}
	if patch.ListURL != nil {
		set["list_url"] = *patch.ListURL
	}
	if patch.FeedURL != nil {
		set["feed_url"] = *patch.FeedURL
	}
	if patch.JSONURL != nil {
		set["json_url"] = *patch.JSONURL
	}
	if patch.JSONPath != nil {
		set["json_path"] = *patch.JSONPath
	}
	if patch.ItemSelector != nil {
		set["item_selector"] = *patch.ItemSelector
	}
	if patch.TitleSelector != nil {
		set["title_selector"] = *patch.TitleSelector
	}
	if patch.LinkAttr != nil {
		set["link_attr"] = *patch.LinkAttr
	}
	if patch.DateSelector != nil {
		set["date_selector"] = *patch.DateSelector
	}
	if patch.NumberPattern != nil {
		set["number_pattern"] = *patch.NumberPattern
	}

	var updated domain.Source
	err := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrSourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.Update: %w", err)
	}
	return &updated, nil
}

// Delete removes a source. Deleting a missing source is an error, so a mistyped identifier is not
// reported as success.
func (r *MongoSourceRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("repository.Delete: %w", err)
	}
	if result.DeletedCount == 0 {
		return domain.ErrSourceNotFound
	}
	return nil
}
