package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/erivelto/read-tracker/tracker/domain"
	"github.com/erivelto/read-tracker/tracker/usecase"
)

// mockTitleRepo is a test double for usecase.TitleRepository.
type mockTitleRepo struct {
	findByNameFn func(ctx context.Context, name string) (*domain.Title, error)
	saveFn       func(ctx context.Context, title *domain.Title) (*domain.Title, error)
}

func (m *mockTitleRepo) FindByName(ctx context.Context, name string) (*domain.Title, error) {
	return m.findByNameFn(ctx, name)
}

func (m *mockTitleRepo) Save(ctx context.Context, title *domain.Title) (*domain.Title, error) {
	return m.saveFn(ctx, title)
}

func TestTitleUsecase_Create(t *testing.T) {
	t.Parallel()

	title := &domain.Title{Name: "Berserk", Type: domain.Manga}
	repoErr := errors.New("db error")

	tests := []struct {
		name        string
		findByName  func(ctx context.Context, n string) (*domain.Title, error)
		save        func(ctx context.Context, t *domain.Title) (*domain.Title, error)
		wantErr     error
		wantNilData bool
	}{
		{
			name: "success",
			findByName: func(_ context.Context, _ string) (*domain.Title, error) {
				return nil, domain.ErrNotFound
			},
			save: func(_ context.Context, t *domain.Title) (*domain.Title, error) {
				return t, nil
			},
			wantErr: nil,
		},
		{
			name: "duplicate name",
			findByName: func(_ context.Context, _ string) (*domain.Title, error) {
				return &domain.Title{Name: "Berserk"}, nil
			},
			save: func(_ context.Context, t *domain.Title) (*domain.Title, error) {
				return t, nil // should not be called
			},
			wantErr:     domain.ErrAlreadyExists,
			wantNilData: true,
		},
		{
			name: "repository save error",
			findByName: func(_ context.Context, _ string) (*domain.Title, error) {
				return nil, domain.ErrNotFound
			},
			save: func(_ context.Context, _ *domain.Title) (*domain.Title, error) {
				return nil, repoErr
			},
			wantErr:     repoErr,
			wantNilData: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &mockTitleRepo{
				findByNameFn: tt.findByName,
				saveFn:       tt.save,
			}
			uc := usecase.NewTitleUsecase(repo)

			got, err := uc.Create(context.Background(), title)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Create() unexpected error = %v", err)
			}

			if tt.wantNilData && got != nil {
				t.Errorf("Create() expected nil result, got %v", got)
			}
			if !tt.wantNilData && err == nil {
				if got == nil {
					t.Error("Create() expected non-nil result")
				} else if got.ExternalID == "" {
					t.Error("Create() expected ExternalID to be set, got empty string")
				}
			}
		})
	}
}
