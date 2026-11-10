package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// fakeSourceRepo is a test double for usecase.SourceRepository.
type fakeSourceRepo struct {
	findAll    func(ctx context.Context) ([]domain.Source, error)
	findByID   func(ctx context.Context, id bson.ObjectID) (*domain.Source, error)
	findByPair func(ctx context.Context, host, pattern string, excludeID *bson.ObjectID) (*domain.Source, error)
	insert     func(ctx context.Context, source *domain.Source) (*domain.Source, error)
	update     func(ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, at time.Time, by string) (*domain.Source, error)
	deleteFn   func(ctx context.Context, id bson.ObjectID) error
}

func (f *fakeSourceRepo) FindAll(ctx context.Context) ([]domain.Source, error) {
	return f.findAll(ctx)
}

func (f *fakeSourceRepo) FindByID(ctx context.Context, id bson.ObjectID) (*domain.Source, error) {
	return f.findByID(ctx, id)
}

func (f *fakeSourceRepo) FindByHostAndPattern(
	ctx context.Context, host, pattern string, excludeID *bson.ObjectID,
) (*domain.Source, error) {
	return f.findByPair(ctx, host, pattern, excludeID)
}

func (f *fakeSourceRepo) Insert(ctx context.Context, source *domain.Source) (*domain.Source, error) {
	return f.insert(ctx, source)
}

func (f *fakeSourceRepo) Update(
	ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, at time.Time, by string,
) (*domain.Source, error) {
	return f.update(ctx, id, patch, at, by)
}

func (f *fakeSourceRepo) Delete(ctx context.Context, id bson.ObjectID) error {
	return f.deleteFn(ctx, id)
}

// sourceRepoWithNoConflict is a repository where every lookup misses, which is the common case
// for the tests that are not about uniqueness.
func sourceRepoWithNoConflict() *fakeSourceRepo {
	return &fakeSourceRepo{
		findAll: func(context.Context) ([]domain.Source, error) { return nil, nil },
		findByID: func(context.Context, bson.ObjectID) (*domain.Source, error) {
			return nil, domain.ErrSourceNotFound
		},
		findByPair: func(context.Context, string, string, *bson.ObjectID) (*domain.Source, error) {
			return nil, domain.ErrSourceNotFound
		},
	}
}

func validSource() domain.Source {
	return domain.Source{
		Host:       "example.com",
		Name:       "Example",
		URLPattern: `^https://example\.com/manga/[^/]+$`,
		Strategy:   domain.SourceStrategyFeed,
		Profile:    "chrome",
		FeedURL:    "https://example.com/feed.xml",
	}
}

func TestSourceCreateStampsProvenance(t *testing.T) {
	t.Parallel()

	var inserted *domain.Source
	repo := sourceRepoWithNoConflict()
	repo.insert = func(_ context.Context, source *domain.Source) (*domain.Source, error) {
		source.ID = bson.NewObjectID()
		inserted = source
		return source, nil
	}

	uc := NewSourceUsecase(repo)
	uc.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

	if _, err := uc.Create(context.Background(), validSource(), "operator"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inserted == nil {
		t.Fatal("expected the source to be inserted")
	}
	if inserted.CreatedAt.IsZero() || inserted.UpdatedAt.IsZero() {
		t.Error("expected created_at and updated_at to be stamped")
	}
	if inserted.UpdatedBy != "operator" {
		t.Errorf("expected updated_by %q, got %q", "operator", inserted.UpdatedBy)
	}
}

func TestSourceCreateRejectsAnInvalidSourceWithoutWriting(t *testing.T) {
	t.Parallel()

	inserted := false
	repo := sourceRepoWithNoConflict()
	repo.insert = func(context.Context, *domain.Source) (*domain.Source, error) {
		inserted = true
		return nil, nil
	}

	source := validSource()
	source.FeedURL = ""

	_, err := NewSourceUsecase(repo).Create(context.Background(), source, "operator")

	var configErr *domain.SourceConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("expected a *SourceConfigError, got %T: %v", err, err)
	}
	if configErr.Field != "feedUrl" {
		t.Errorf("expected the error to name feedUrl, got %q", configErr.Field)
	}
	if inserted {
		t.Error("an invalid source must never reach the repository")
	}
}

func TestSourceCreateRejectsADuplicateHostAndPattern(t *testing.T) {
	t.Parallel()

	inserted := false
	repo := sourceRepoWithNoConflict()
	repo.findByPair = func(context.Context, string, string, *bson.ObjectID) (*domain.Source, error) {
		return &domain.Source{ID: bson.NewObjectID()}, nil
	}
	repo.insert = func(context.Context, *domain.Source) (*domain.Source, error) {
		inserted = true
		return nil, nil
	}

	_, err := NewSourceUsecase(repo).Create(context.Background(), validSource(), "operator")

	if !errors.Is(err, domain.ErrSourceExists) {
		t.Fatalf("expected ErrSourceExists, got %v", err)
	}
	if inserted {
		t.Error("a duplicate source must never reach the repository")
	}
}

func TestSourceUpdateValidatesTheMergedDocument(t *testing.T) {
	t.Parallel()

	// Switching to the css strategy without supplying its selectors must be refused: the stored
	// document would otherwise be one the crawler rejects, and the failure would only surface on
	// the next run.
	repo := sourceRepoWithNoConflict()
	repo.findByID = func(context.Context, bson.ObjectID) (*domain.Source, error) {
		current := validSource()
		current.ID = bson.NewObjectID()
		return &current, nil
	}
	repo.update = func(context.Context, bson.ObjectID, domain.SourcePatch, time.Time, string) (*domain.Source, error) {
		t.Fatal("an invalid merge must not be written")
		return nil, nil
	}

	css := domain.SourceStrategyCSS
	patch := domain.SourcePatch{Strategy: &css}

	_, err := NewSourceUsecase(repo).Update(context.Background(), bson.NewObjectID(), patch, "operator")

	var configErr *domain.SourceConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("expected a *SourceConfigError, got %T: %v", err, err)
	}
}

func TestSourceUpdateAcceptsAValidMerge(t *testing.T) {
	t.Parallel()

	id := bson.NewObjectID()
	var written domain.SourcePatch

	repo := sourceRepoWithNoConflict()
	repo.findByID = func(context.Context, bson.ObjectID) (*domain.Source, error) {
		current := validSource()
		current.ID = id
		return &current, nil
	}
	repo.update = func(_ context.Context, _ bson.ObjectID, patch domain.SourcePatch, _ time.Time, _ string) (*domain.Source, error) {
		written = patch
		updated := validSource()
		updated.ID = id
		return &updated, nil
	}

	enabled := false
	patch := domain.SourcePatch{Enabled: &enabled}

	if _, err := NewSourceUsecase(repo).Update(context.Background(), id, patch, "operator"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written.Enabled == nil || *written.Enabled {
		t.Error("expected the enabled patch to reach the repository")
	}
}

func TestSourceUpdateRejectsAnEmptyPatch(t *testing.T) {
	t.Parallel()

	repo := sourceRepoWithNoConflict()
	repo.update = func(context.Context, bson.ObjectID, domain.SourcePatch, time.Time, string) (*domain.Source, error) {
		t.Fatal("an empty patch must not be written")
		return nil, nil
	}

	_, err := NewSourceUsecase(repo).Update(context.Background(), bson.NewObjectID(), domain.SourcePatch{}, "operator")

	if !errors.Is(err, domain.ErrEmptyPatch) {
		t.Fatalf("expected ErrEmptyPatch, got %v", err)
	}
}

func TestSourceUpdateReportsAMissingSource(t *testing.T) {
	t.Parallel()

	repo := sourceRepoWithNoConflict()

	_, err := NewSourceUsecase(repo).Update(
		context.Background(), bson.NewObjectID(), domain.SourcePatch{}, "operator",
	)
	if err == nil {
		t.Fatal("expected an error for a missing source")
	}
}

func TestSourceDeletePropagatesNotFound(t *testing.T) {
	t.Parallel()

	repo := sourceRepoWithNoConflict()
	repo.deleteFn = func(context.Context, bson.ObjectID) error {
		return domain.ErrSourceNotFound
	}

	err := NewSourceUsecase(repo).Delete(context.Background(), bson.NewObjectID())

	if !errors.Is(err, domain.ErrSourceNotFound) {
		t.Fatalf("expected ErrSourceNotFound, got %v", err)
	}
}
