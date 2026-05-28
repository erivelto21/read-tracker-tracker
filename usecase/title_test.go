package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/erivelto/read-tracker/tracker/domain"
	"github.com/erivelto/read-tracker/tracker/usecase"
)

// fakeTitleRepo is a test double for usecase.TitleRepository.
type fakeTitleRepo struct {
	findByNameFn          func(ctx context.Context, name string) (*domain.Title, error)
	findByExternalIDFn    func(ctx context.Context, externalID string) (*domain.Title, error)
	findAllFn             func(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error)
	saveFn                func(ctx context.Context, title *domain.Title) (*domain.Title, error)
	updateFn              func(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error)
	deleteByExternalIDFn  func(ctx context.Context, externalID string) error
}

func (m *fakeTitleRepo) FindByName(ctx context.Context, name string) (*domain.Title, error) {
	return m.findByNameFn(ctx, name)
}

func (m *fakeTitleRepo) FindByExternalID(ctx context.Context, externalID string) (*domain.Title, error) {
	if m.findByExternalIDFn != nil {
		return m.findByExternalIDFn(ctx, externalID)
	}
	return nil, domain.ErrNotFound
}

func (m *fakeTitleRepo) FindAll(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error) {
	return m.findAllFn(ctx, filter)
}

func (m *fakeTitleRepo) Save(ctx context.Context, title *domain.Title) (*domain.Title, error) {
	return m.saveFn(ctx, title)
}

func (m *fakeTitleRepo) Update(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, externalID, fields)
	}
	return nil, nil
}

func (m *fakeTitleRepo) DeleteByExternalID(ctx context.Context, externalID string) error {
	if m.deleteByExternalIDFn != nil {
		return m.deleteByExternalIDFn(ctx, externalID)
	}
	return nil
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

			repo := &fakeTitleRepo{
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

func TestTitleUsecase_List(t *testing.T) {
t.Parallel()

manga := domain.Manga
titles := []domain.Title{
{ExternalID: "abc", Name: "Berserk", Type: domain.Manga},
}
repoErr := errors.New("db error")

tests := []struct {
name        string
filter      domain.TitleFilter
findAllFn   func(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error)
wantErr     error
wantDataLen int
}{
{
name:   "delegates filter to repository",
filter: domain.TitleFilter{Type: &manga},
findAllFn: func(_ context.Context, f domain.TitleFilter) ([]domain.Title, error) {
if f.Type == nil || *f.Type != domain.Manga {
t.Errorf("expected manga filter, got %v", f.Type)
}
return titles, nil
},
wantDataLen: 1,
},
{
name:   "returns empty slice when no matches",
filter: domain.TitleFilter{},
findAllFn: func(_ context.Context, _ domain.TitleFilter) ([]domain.Title, error) {
return []domain.Title{}, nil
},
wantDataLen: 0,
},
{
name:   "propagates repository error",
filter: domain.TitleFilter{},
findAllFn: func(_ context.Context, _ domain.TitleFilter) ([]domain.Title, error) {
return nil, repoErr
},
wantErr: repoErr,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
t.Parallel()

repo := &fakeTitleRepo{
findByNameFn: func(_ context.Context, _ string) (*domain.Title, error) {
return nil, domain.ErrNotFound
},
findAllFn: tt.findAllFn,
saveFn: func(_ context.Context, t *domain.Title) (*domain.Title, error) {
return t, nil
},
}
uc := usecase.NewTitleUsecase(repo)

got, err := uc.List(context.Background(), tt.filter)

if tt.wantErr != nil {
if !errors.Is(err, tt.wantErr) {
t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
}
return
}
if err != nil {
t.Errorf("List() unexpected error = %v", err)
}
if len(got) != tt.wantDataLen {
t.Errorf("List() len = %d, want %d", len(got), tt.wantDataLen)
}
})
}
}

func TestTitleUsecase_Update(t *testing.T) {
t.Parallel()

chapter := 10
existing := &domain.Title{ExternalID: "uuid-1", Name: "Berserk", Type: domain.Manga}
repoErr := errors.New("db error")

tests := []struct {
name               string
findByExternalIDFn func(ctx context.Context, id string) (*domain.Title, error)
updateFn           func(ctx context.Context, id string, f domain.TitleUpdate) (*domain.Title, error)
wantErr            error
wantNilData        bool
}{
{
	name: "success",
	findByExternalIDFn: func(_ context.Context, _ string) (*domain.Title, error) {
		return existing, nil
	},
	updateFn: func(_ context.Context, _ string, f domain.TitleUpdate) (*domain.Title, error) {
		updated := *existing
		updated.Chapter = f.Chapter
		return &updated, nil
	},
},
{
	name: "title not found",
	findByExternalIDFn: func(_ context.Context, _ string) (*domain.Title, error) {
		return nil, domain.ErrNotFound
	},
	wantErr:     domain.ErrNotFound,
	wantNilData: true,
},
{
	name: "repository update error",
	findByExternalIDFn: func(_ context.Context, _ string) (*domain.Title, error) {
		return existing, nil
	},
	updateFn: func(_ context.Context, _ string, _ domain.TitleUpdate) (*domain.Title, error) {
		return nil, repoErr
	},
	wantErr:     repoErr,
	wantNilData: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
	t.Parallel()

	repo := &fakeTitleRepo{
		findByNameFn: func(_ context.Context, _ string) (*domain.Title, error) {
			return nil, domain.ErrNotFound
		},
		findByExternalIDFn: tt.findByExternalIDFn,
		saveFn: func(_ context.Context, t *domain.Title) (*domain.Title, error) {
			return t, nil
		},
		updateFn: tt.updateFn,
	}
	uc := usecase.NewTitleUsecase(repo)

	fields := domain.TitleUpdate{Chapter: &chapter}
	got, err := uc.Update(context.Background(), "uuid-1", fields)

	if tt.wantErr != nil {
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
		}
	} else if err != nil {
		t.Errorf("Update() unexpected error = %v", err)
	}

	if tt.wantNilData && got != nil {
		t.Errorf("Update() expected nil result, got %v", got)
	}
	if !tt.wantNilData && err == nil && got == nil {
		t.Error("Update() expected non-nil result")
	}
})
}
}

func TestTitleUsecase_Delete(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("db error")

	tests := []struct {
		name                 string
		deleteByExternalIDFn func(ctx context.Context, id string) error
		wantErr              error
	}{
		{
			name: "deletes existing title",
			deleteByExternalIDFn: func(_ context.Context, _ string) error {
				return nil
			},
			wantErr: nil,
		},
		{
			name: "non-existent title is a no-op",
			deleteByExternalIDFn: func(_ context.Context, _ string) error {
				return nil
			},
			wantErr: nil,
		},
		{
			name: "repository error is propagated",
			deleteByExternalIDFn: func(_ context.Context, _ string) error {
				return repoErr
			},
			wantErr: repoErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeTitleRepo{
				deleteByExternalIDFn: tt.deleteByExternalIDFn,
			}
			uc := usecase.NewTitleUsecase(repo)

			err := uc.Delete(context.Background(), "some-id")

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Delete() unexpected error = %v", err)
			}
		})
	}
}
