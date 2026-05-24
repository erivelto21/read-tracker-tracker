package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// MongoTitleRepository implements domain title persistence using MongoDB.
type MongoTitleRepository struct {
	collection *mongo.Collection
}

// NewMongoTitleRepository returns a new MongoTitleRepository.
func NewMongoTitleRepository(col *mongo.Collection) *MongoTitleRepository {
	return &MongoTitleRepository{collection: col}
}

// FindByName retrieves a title by its name. Returns domain.ErrNotFound if it does not exist.
func (r *MongoTitleRepository) FindByName(ctx context.Context, name string) (*domain.Title, error) {
	var title domain.Title
	err := r.collection.FindOne(ctx, bson.M{"name": name}).Decode(&title)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.FindByName: %w", err)
	}
	return &title, nil
}

// FindAll retrieves all titles matching the given filter. Returns an empty non-nil slice when no
// documents match.
func (r *MongoTitleRepository) FindAll(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error) {
	query := bson.M{}
	if filter.Type != nil {
		query["type"] = *filter.Type
	}
	if filter.Name != nil {
		query["name"] = bson.M{"$regex": *filter.Name, "$options": "i"}
	}

	cursor, err := r.collection.Find(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAll: %w", err)
	}

	titles := make([]domain.Title, 0)
	if err := cursor.All(ctx, &titles); err != nil {
		return nil, fmt.Errorf("repository.FindAll: %w", err)
	}
	return titles, nil
}

// Save persists a new title. Maps MongoDB duplicate-key errors (E11000) to domain.ErrAlreadyExists.
func (r *MongoTitleRepository) Save(ctx context.Context, title *domain.Title) (*domain.Title, error) {
	_, err := r.collection.InsertOne(ctx, title)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, fmt.Errorf("repository.Save: %w", err)
	}
	return title, nil
}

// FindByExternalID retrieves a title by its public UUID. Returns domain.ErrNotFound if it does not exist.
func (r *MongoTitleRepository) FindByExternalID(ctx context.Context, externalID string) (*domain.Title, error) {
	var title domain.Title
	err := r.collection.FindOne(ctx, bson.M{"external_id": externalID}).Decode(&title)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.FindByExternalID: %w", err)
	}
	return &title, nil
}

// Update applies a partial update to the title identified by externalID.
// Only non-nil fields in fields are written. Returns domain.ErrNotFound if no document matches.
func (r *MongoTitleRepository) Update(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error) {
	set := bson.M{}
	if fields.Chapter != nil {
		set["chapter"] = *fields.Chapter
	}
	if fields.Page != nil {
		set["page"] = *fields.Page
	}
	if fields.Link != nil {
		set["link"] = *fields.Link
	}
	if fields.Observation != nil {
		set["observation"] = *fields.Observation
	}

	after := options.After
	var updated domain.Title
	err := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"external_id": externalID},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(after),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository.Update: %w", err)
	}
	return &updated, nil
}
