package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// TitleRepository defines the persistence contract required by the title usecase.
type TitleRepository interface {
	FindByName(ctx context.Context, name string) (*domain.Title, error)
	FindByExternalID(ctx context.Context, externalID string) (*domain.Title, error)
	FindAll(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error)
	Save(ctx context.Context, title *domain.Title) (*domain.Title, error)
	Update(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error)
	DeleteByExternalID(ctx context.Context, externalID string) error
}

// TitleUsecase encapsulates business logic for title operations.
type TitleUsecase struct {
	repo TitleRepository
}

// NewTitleUsecase returns a new TitleUsecase.
func NewTitleUsecase(repo TitleRepository) *TitleUsecase {
	return &TitleUsecase{repo: repo}
}

// Create persists a new title, enforcing name uniqueness before delegating to the repository.
func (u *TitleUsecase) Create(ctx context.Context, title *domain.Title) (*domain.Title, error) {
	_, err := u.repo.FindByName(ctx, title.Name)
	if err == nil {
		return nil, domain.ErrAlreadyExists
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("usecase.Create: %w", err)
	}

	title.ExternalID = uuid.NewString()

	saved, err := u.repo.Save(ctx, title)
	if err != nil {
		return nil, fmt.Errorf("usecase.Create: %w", err)
	}
	return saved, nil
}

// List returns all titles matching the given filter.
func (u *TitleUsecase) List(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error) {
	return u.repo.FindAll(ctx, filter)
}

// Delete permanently removes the title identified by externalID.
func (u *TitleUsecase) Delete(ctx context.Context, externalID string) error {
	_, err := u.repo.FindByExternalID(ctx, externalID)
	if err != nil {
		return fmt.Errorf("usecase.Delete: %w", err)
	}

	if err := u.repo.DeleteByExternalID(ctx, externalID); err != nil {
		return fmt.Errorf("usecase.Delete: %w", err)
	}
	return nil
}

// Update partially updates the mutable fields of the title identified by externalID.
func (u *TitleUsecase) Update(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error) {
	_, err := u.repo.FindByExternalID(ctx, externalID)
	if err != nil {
		return nil, fmt.Errorf("usecase.Update: %w", err)
	}

	updated, err := u.repo.Update(ctx, externalID, fields)
	if err != nil {
		return nil, fmt.Errorf("usecase.Update: %w", err)
	}
	return updated, nil
}
