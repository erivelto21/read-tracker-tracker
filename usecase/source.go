package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// SourceRepository defines the persistence contract required by the source usecase.
type SourceRepository interface {
	FindAll(ctx context.Context) ([]domain.Source, error)
	FindByID(ctx context.Context, id bson.ObjectID) (*domain.Source, error)
	FindByHostAndPattern(ctx context.Context, host, pattern string, excludeID *bson.ObjectID) (*domain.Source, error)
	Insert(ctx context.Context, source *domain.Source) (*domain.Source, error)
	Update(ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, at time.Time, by string) (*domain.Source, error)
	Delete(ctx context.Context, id bson.ObjectID) error
}

// SourceUsecase manages the crawler's source definitions.
//
// A source written here is picked up by the next crawl. Validation is performed before every
// write, so a document that would make the crawler report a misconfigured source is refused at
// the door instead of being stored and silently ignored.
type SourceUsecase struct {
	repo SourceRepository
	now  func() time.Time
}

// NewSourceUsecase returns a new SourceUsecase.
func NewSourceUsecase(repo SourceRepository) *SourceUsecase {
	return &SourceUsecase{repo: repo, now: time.Now}
}

// List returns every source, configuration and health together.
func (u *SourceUsecase) List(ctx context.Context) ([]domain.Source, error) {
	sources, err := u.repo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("usecase.List: %w", err)
	}
	return sources, nil
}

// Get returns one source.
func (u *SourceUsecase) Get(ctx context.Context, id bson.ObjectID) (*domain.Source, error) {
	source, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("usecase.Get: %w", err)
	}
	return source, nil
}

// Create validates and stores a new source.
func (u *SourceUsecase) Create(ctx context.Context, source domain.Source, by string) (*domain.Source, error) {
	if err := domain.ValidateSource(source); err != nil {
		return nil, fmt.Errorf("usecase.Create: %w", err)
	}

	if err := u.ensureUnique(ctx, source.Host, source.URLPattern, nil); err != nil {
		return nil, err
	}

	now := u.now()
	source.CreatedAt = now
	source.UpdatedAt = now
	source.UpdatedBy = by

	created, err := u.repo.Insert(ctx, &source)
	if err != nil {
		return nil, fmt.Errorf("usecase.Create: %w", err)
	}
	return created, nil
}

// Update validates the merged result of the patch and stores it.
//
// The validation runs against the merged document rather than the patch alone: switching a
// source from feed to css while sending only the new strategy would otherwise be accepted and
// produce a document the crawler rejects.
func (u *SourceUsecase) Update(
	ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, by string,
) (*domain.Source, error) {
	if patch.IsEmpty() {
		return nil, fmt.Errorf("usecase.Update: %w", domain.ErrEmptyPatch)
	}

	current, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("usecase.Update: %w", err)
	}

	merged := patch.Apply(*current)
	if err := domain.ValidateSource(merged); err != nil {
		return nil, fmt.Errorf("usecase.Update: %w", err)
	}

	if err := u.ensureUnique(ctx, merged.Host, merged.URLPattern, &id); err != nil {
		return nil, err
	}

	updated, err := u.repo.Update(ctx, id, patch, u.now(), by)
	if err != nil {
		return nil, fmt.Errorf("usecase.Update: %w", err)
	}
	return updated, nil
}

// Delete removes a source.
func (u *SourceUsecase) Delete(ctx context.Context, id bson.ObjectID) error {
	if err := u.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("usecase.Delete: %w", err)
	}
	return nil
}

// ensureUnique rejects a second source claiming the same host and URL pattern, which would make
// the crawler's match order the only thing deciding which one is used.
func (u *SourceUsecase) ensureUnique(
	ctx context.Context, host, pattern string, excludeID *bson.ObjectID,
) error {
	_, err := u.repo.FindByHostAndPattern(ctx, host, pattern, excludeID)
	if err == nil {
		return domain.ErrSourceExists
	}
	if !errors.Is(err, domain.ErrSourceNotFound) {
		return fmt.Errorf("usecase.ensureUnique: %w", err)
	}
	return nil
}
