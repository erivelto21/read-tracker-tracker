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
	FindAll(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error)
	Save(ctx context.Context, title *domain.Title) (*domain.Title, error)
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
