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
