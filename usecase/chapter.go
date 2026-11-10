package usecase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
	"github.com/erivelto/read-tracker/tracker/repository"
)

// ChapterRepository defines the persistence contract required by the chapter usecase.
type ChapterRepository interface {
	FindAll(ctx context.Context, filter domain.ChapterFilter) ([]domain.Chapter, error)
	FindByID(ctx context.Context, id bson.ObjectID) (*domain.Chapter, error)
	MarkRead(ctx context.Context, id bson.ObjectID, at time.Time) (*domain.Chapter, error)
	UnreadCounts(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error)
}

// ChapterTitleRepository is the title lookup the chapter usecase needs, narrowed to what it uses.
type ChapterTitleRepository interface {
	FindByInternalID(ctx context.Context, id bson.ObjectID) (*domain.Title, error)
	Update(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error)
}

// ChapterUsecase encapsulates business logic for chapters.
type ChapterUsecase struct {
	chapters ChapterRepository
	titles   ChapterTitleRepository
	now      func() time.Time
}

// NewChapterUsecase returns a new ChapterUsecase.
func NewChapterUsecase(
	chapters ChapterRepository, titles ChapterTitleRepository, now func() time.Time,
) *ChapterUsecase {
	if now == nil {
		now = time.Now
	}
	return &ChapterUsecase{chapters: chapters, titles: titles, now: now}
}

// ChapterPage is one page of chapters plus the cursor that continues after it.
type ChapterPage struct {
	Chapters   []domain.ChapterView
	NextCursor string
}

// List returns chapters matching the filter, enriched with their title's name.
func (u *ChapterUsecase) List(ctx context.Context, filter domain.ChapterFilter) (ChapterPage, error) {
	chapters, err := u.chapters.FindAll(ctx, filter)
	if err != nil {
		return ChapterPage{}, fmt.Errorf("usecase.List: %w", err)
	}

	names := make(map[bson.ObjectID]string)
	views := make([]domain.ChapterView, 0, len(chapters))

	for _, chapter := range chapters {
		name, cached := names[chapter.TitleID]
		if !cached {
			title, titleErr := u.titles.FindByInternalID(ctx, chapter.TitleID)
			if titleErr == nil {
				name = title.Name
			}
			names[chapter.TitleID] = name
		}
		views = append(views, toView(chapter, name))
	}

	page := ChapterPage{Chapters: views}
	if filter.Limit > 0 && len(chapters) == filter.Limit {
		last := chapters[len(chapters)-1]
		page.NextCursor = repository.EncodeCursor(last)
	}

	return page, nil
}

// MarkRead marks a chapter read and advances the owning title's progress when the chapter number
// is higher than the title's current one.
func (u *ChapterUsecase) MarkRead(ctx context.Context, id bson.ObjectID) (*domain.ChapterView, error) {
	chapter, err := u.chapters.MarkRead(ctx, id, u.now())
	if err != nil {
		return nil, fmt.Errorf("usecase.MarkRead: %w", err)
	}

	title, err := u.titles.FindByInternalID(ctx, chapter.TitleID)
	if err != nil {
		// The chapter is marked read either way; failing the whole request because the title
		// lookup failed would leave the client believing nothing happened.
		return &domain.ChapterView{ID: chapter.ID.Hex(), TitleID: chapter.TitleID.Hex()}, nil
	}

	if shouldAdvance(title.Chapter, chapter.Number) {
		next := int(*chapter.Number)
		if _, updateErr := u.titles.Update(ctx, title.ExternalID, domain.TitleUpdate{Chapter: &next}); updateErr != nil {
			return nil, fmt.Errorf("usecase.MarkRead: advance title: %w", updateErr)
		}
	}

	return toViewPointer(chapter, title.Name), nil
}

// ParseLimit validates a client-supplied page size, clamping it to a sane range.
func ParseLimit(raw string) (int, error) {
	if raw == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid limit %q", raw)
	}
	if limit < 1 {
		return 0, fmt.Errorf("invalid limit %q: must be at least 1", raw)
	}
	if limit > 200 {
		return 200, nil
	}
	return limit, nil
}

// shouldAdvance reports whether reading this chapter should move the title's progress forward.
// A chapter with no number never advances it, and reading behind never regresses it.
func shouldAdvance(current *int, number *float64) bool {
	if number == nil {
		return false
	}
	next := int(*number)
	if current == nil {
		return true
	}
	return next > *current
}

// UnreadCount returns the unread counts, optionally scoped to one title.
func (u *ChapterUsecase) UnreadCount(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error) {
	counts, err := u.chapters.UnreadCounts(ctx, titleID)
	if err != nil {
		return domain.UnreadCount{}, fmt.Errorf("usecase.UnreadCount: %w", err)
	}
	return counts, nil
}

func toView(chapter domain.Chapter, titleName string) domain.ChapterView {
	return domain.ChapterView{
		ID:           chapter.ID.Hex(),
		TitleID:      chapter.TitleID.Hex(),
		TitleName:    titleName,
		Host:         chapter.Host,
		Number:       chapter.Number,
		NumberRaw:    chapter.NumberRaw,
		Name:         chapter.Name,
		URL:          chapter.URL,
		PublishedAt:  chapter.PublishedAt,
		DiscoveredAt: chapter.DiscoveredAt,
		ReadAt:       chapter.ReadAt,
		Imported:     chapter.Imported,
	}
}

func toViewPointer(chapter *domain.Chapter, titleName string) *domain.ChapterView {
	view := toView(*chapter, titleName)
	return &view
}
