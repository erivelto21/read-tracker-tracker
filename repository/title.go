package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

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
