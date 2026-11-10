package usecase

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
)

func ptrInt(n int) *int           { return &n }
func ptrFloat(n float64) *float64 { return &n }

type fakeChapterRepo struct {
	findAll     func(ctx context.Context, filter domain.ChapterFilter) ([]domain.Chapter, error)
	findByID    func(ctx context.Context, id bson.ObjectID) (*domain.Chapter, error)
	markRead    func(ctx context.Context, id bson.ObjectID, at time.Time) (*domain.Chapter, error)
	unreadCount func(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error)
}

func (f *fakeChapterRepo) FindAll(ctx context.Context, filter domain.ChapterFilter) ([]domain.Chapter, error) {
	return f.findAll(ctx, filter)
}

func (f *fakeChapterRepo) FindByID(ctx context.Context, id bson.ObjectID) (*domain.Chapter, error) {
	return f.findByID(ctx, id)
}

func (f *fakeChapterRepo) MarkRead(ctx context.Context, id bson.ObjectID, at time.Time) (*domain.Chapter, error) {
	return f.markRead(ctx, id, at)
}

func (f *fakeChapterRepo) UnreadCounts(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error) {
	return f.unreadCount(ctx, titleID)
}

type fakeTitleRepo struct {
	title     *domain.Title
	findErr   error
	updates   []domain.TitleUpdate
	updateErr error
}

func (f *fakeTitleRepo) FindByInternalID(_ context.Context, _ bson.ObjectID) (*domain.Title, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.title, nil
}

func (f *fakeTitleRepo) Update(_ context.Context, _ string, fields domain.TitleUpdate) (*domain.Title, error) {
	f.updates = append(f.updates, fields)
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.title, nil
}

func TestShouldAdvance(t *testing.T) {
	tests := []struct {
		name    string
		current *int
		number  *float64
		want    bool
	}{
		{"reading ahead advances progress", ptrInt(87), ptrFloat(88), true},
		{"reading behind does not regress progress", ptrInt(87), ptrFloat(80), false},
		{"reading the same chapter does nothing", ptrInt(87), ptrFloat(87), false},
		{"a non-numeric chapter never advances", ptrInt(87), nil, false},
		{"a non-numeric chapter never advances without progress", nil, nil, false},
		{"the first chapter sets progress", nil, ptrFloat(1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAdvance(tt.current, tt.number); got != tt.want {
				t.Fatalf("shouldAdvance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarkReadAdvancesTitleProgress(t *testing.T) {
	titleID := bson.NewObjectID()
	chapterID := bson.NewObjectID()

	titles := &fakeTitleRepo{title: &domain.Title{
		ExternalID: "title-uuid", Name: "One", Chapter: ptrInt(87),
	}}
	chapters := &fakeChapterRepo{
		markRead: func(_ context.Context, _ bson.ObjectID, at time.Time) (*domain.Chapter, error) {
			return &domain.Chapter{ID: chapterID, TitleID: titleID, Number: ptrFloat(88), ReadAt: &at}, nil
		},
	}

	uc := NewChapterUsecase(chapters, titles, nil)
	if _, err := uc.MarkRead(context.Background(), chapterID); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	if len(titles.updates) != 1 {
		t.Fatalf("expected one title update, got %d", len(titles.updates))
	}
	if titles.updates[0].Chapter == nil || *titles.updates[0].Chapter != 88 {
		t.Fatalf("title chapter = %v, want 88", titles.updates[0].Chapter)
	}
}

func TestMarkReadBehindDoesNotRegressTitle(t *testing.T) {
	titleID := bson.NewObjectID()
	chapterID := bson.NewObjectID()

	titles := &fakeTitleRepo{title: &domain.Title{
		ExternalID: "title-uuid", Name: "One", Chapter: ptrInt(87),
	}}
	chapters := &fakeChapterRepo{
		markRead: func(_ context.Context, _ bson.ObjectID, at time.Time) (*domain.Chapter, error) {
			return &domain.Chapter{ID: chapterID, TitleID: titleID, Number: ptrFloat(80), ReadAt: &at}, nil
		},
	}

	uc := NewChapterUsecase(chapters, titles, nil)
	if _, err := uc.MarkRead(context.Background(), chapterID); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	if len(titles.updates) != 0 {
		t.Fatalf("reading behind must not update the title, got %d updates", len(titles.updates))
	}
}

func TestMarkReadNonNumericDoesNotTouchTitle(t *testing.T) {
	titleID := bson.NewObjectID()
	chapterID := bson.NewObjectID()

	titles := &fakeTitleRepo{title: &domain.Title{
		ExternalID: "title-uuid", Name: "One", Chapter: ptrInt(87),
	}}
	chapters := &fakeChapterRepo{
		markRead: func(_ context.Context, _ bson.ObjectID, at time.Time) (*domain.Chapter, error) {
			return &domain.Chapter{ID: chapterID, TitleID: titleID, Name: "Epilogue", ReadAt: &at}, nil
		},
	}

	uc := NewChapterUsecase(chapters, titles, nil)
	if _, err := uc.MarkRead(context.Background(), chapterID); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	if len(titles.updates) != 0 {
		t.Fatalf("a chapter with no number must not change progress, got %d updates", len(titles.updates))
	}
}

func TestMarkReadReturnsNotFound(t *testing.T) {
	chapters := &fakeChapterRepo{
		markRead: func(context.Context, bson.ObjectID, time.Time) (*domain.Chapter, error) {
			return nil, domain.ErrChapterNotFound
		},
	}

	uc := NewChapterUsecase(chapters, &fakeTitleRepo{}, nil)
	if _, err := uc.MarkRead(context.Background(), bson.NewObjectID()); err == nil {
		t.Fatal("expected ErrChapterNotFound")
	}
}

func TestListEnrichesWithTitleName(t *testing.T) {
	titleID := bson.NewObjectID()

	chapters := &fakeChapterRepo{
		findAll: func(context.Context, domain.ChapterFilter) ([]domain.Chapter, error) {
			return []domain.Chapter{
				{ID: bson.NewObjectID(), TitleID: titleID, Name: "Chapter 1"},
				{ID: bson.NewObjectID(), TitleID: titleID, Name: "Chapter 2"},
			}, nil
		},
	}
	titles := &fakeTitleRepo{title: &domain.Title{ExternalID: "uuid", Name: "One Piece"}}

	uc := NewChapterUsecase(chapters, titles, nil)
	page, err := uc.List(context.Background(), domain.ChapterFilter{Limit: 50})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(page.Chapters) != 2 {
		t.Fatalf("got %d chapters, want 2", len(page.Chapters))
	}
	if page.Chapters[0].TitleName != "One Piece" {
		t.Fatalf("TitleName = %q, want One Piece", page.Chapters[0].TitleName)
	}
}

func TestListEmitsCursorOnlyOnAFullPage(t *testing.T) {
	titleID := bson.NewObjectID()

	build := func(count int) *fakeChapterRepo {
		return &fakeChapterRepo{
			findAll: func(context.Context, domain.ChapterFilter) ([]domain.Chapter, error) {
				out := make([]domain.Chapter, 0, count)
				for i := 0; i < count; i++ {
					out = append(out, domain.Chapter{ID: bson.NewObjectID(), TitleID: titleID})
				}
				return out, nil
			},
		}
	}

	uc := NewChapterUsecase(build(2), &fakeTitleRepo{title: &domain.Title{}}, nil)
	page, err := uc.List(context.Background(), domain.ChapterFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if page.NextCursor == "" {
		t.Fatal("a full page must carry a cursor")
	}

	uc = NewChapterUsecase(build(1), &fakeTitleRepo{title: &domain.Title{}}, nil)
	page, err = uc.List(context.Background(), domain.ChapterFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if page.NextCursor != "" {
		t.Fatal("a partial page must not carry a cursor")
	}
}

func TestParseLimit(t *testing.T) {
	tests := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{"", 50, false},
		{"10", 10, false},
		{"1", 1, false},
		{"200", 200, false},
		{"500", 200, false},
		{"0", 0, true},
		{"-3", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseLimit(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLimit(%q) expected an error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLimit(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("ParseLimit(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
