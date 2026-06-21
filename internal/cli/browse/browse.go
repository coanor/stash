package browse

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

type Query struct {
	Text      string
	Tag       string
	Performer string
	Page      int
	PerPage   int
	Sort      string
	Direction models.SortDirectionEnum
}

type SceneItem struct {
	ID         int
	Title      string
	Path       string
	Duration   float64
	ResumeTime float64
	Date       string
	Studio     string
	Performers []PerformerItem
	Tags       []string
	Rating     *int
	Organized  bool
}

type PerformerItem struct {
	ID          int
	Name        string
	Rating      *int
	BirthYear   *int
	HeightCm    *int
	CareerStart *int
	CareerEnd   *int
	SceneCount  *int
}

type Result struct {
	Items []SceneItem
	Total int
}

type Service struct {
	repo models.Repository
}

func New(repo models.Repository) *Service {
	return &Service{repo: repo}
}

func ParseQuery(raw string) Query {
	var query Query
	for _, part := range strings.Fields(raw) {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			query.Text = strings.TrimSpace(strings.Join([]string{query.Text, part}, " "))
			continue
		}

		switch strings.ToLower(key) {
		case "tag":
			query.Tag = value
		case "performer", "actress":
			query.Performer = value
		default:
			query.Text = strings.TrimSpace(strings.Join([]string{query.Text, part}, " "))
		}
	}

	query.Text = strings.TrimSpace(query.Text)
	return query
}

func (s *Service) Search(ctx context.Context, query Query) (Result, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PerPage == 0 {
		query.PerPage = 40
	}
	if query.Sort == "" {
		query.Sort = "title"
	}
	if query.Direction == "" {
		query.Direction = models.SortDirectionEnumAsc
	}

	var ret Result
	err := s.repo.WithReadTxn(ctx, func(ctx context.Context) error {
		sceneFilter, err := s.buildSceneFilter(ctx, query)
		if err != nil {
			return err
		}

		findFilter := &models.FindFilterType{
			Page:      &query.Page,
			PerPage:   &query.PerPage,
			Sort:      &query.Sort,
			Direction: &query.Direction,
		}
		if query.Text != "" {
			findFilter.Q = &query.Text
		}

		result, err := s.repo.Scene.Query(ctx, models.SceneQueryOptions{
			QueryOptions: models.QueryOptions{
				FindFilter: findFilter,
				Count:      true,
			},
			SceneFilter: sceneFilter,
		})
		if err != nil {
			return err
		}

		scenes, err := result.Resolve(ctx)
		if err != nil {
			return err
		}

		ret.Total = result.Count
		ret.Items = make([]SceneItem, 0, len(scenes))
		cache, err := s.newSceneItemCache(ctx, scenes)
		if err != nil {
			return err
		}
		for _, scene := range scenes {
			item, err := s.sceneItem(ctx, scene, cache)
			if err != nil {
				return err
			}
			ret.Items = append(ret.Items, item)
		}

		return nil
	})

	return ret, err
}

func (s *Service) buildSceneFilter(ctx context.Context, query Query) (*models.SceneFilterType, error) {
	filter := &models.SceneFilterType{}

	if query.Tag != "" {
		tag, err := s.repo.Tag.FindByName(ctx, query.Tag, true)
		if err != nil {
			return nil, fmt.Errorf("find tag %q: %w", query.Tag, err)
		}
		if tag == nil {
			filter.Tags = &models.HierarchicalMultiCriterionInput{
				Value:    []string{"-1"},
				Modifier: models.CriterionModifierIncludes,
			}
		} else {
			filter.Tags = &models.HierarchicalMultiCriterionInput{
				Value:    []string{strconv.Itoa(tag.ID)},
				Modifier: models.CriterionModifierIncludes,
			}
		}
	}

	if query.Performer != "" {
		performers, err := s.repo.Performer.FindByNames(ctx, []string{query.Performer}, true)
		if err != nil {
			return nil, fmt.Errorf("find performer %q: %w", query.Performer, err)
		}

		values := make([]string, 0, len(performers))
		for _, performer := range performers {
			values = append(values, strconv.Itoa(performer.ID))
		}
		if len(values) == 0 {
			values = []string{"-1"}
		}

		filter.Performers = &models.MultiCriterionInput{
			Value:    values,
			Modifier: models.CriterionModifierIncludes,
		}
	}

	if filter.Tags == nil && filter.Performers == nil {
		return nil, nil
	}

	return filter, nil
}

type sceneItemCache struct {
	primaryFiles         map[int]*models.VideoFile
	performerSceneCounts map[int]int
}

func (s *Service) newSceneItemCache(ctx context.Context, scenes []*models.Scene) (*sceneItemCache, error) {
	cache := &sceneItemCache{
		primaryFiles:         map[int]*models.VideoFile{},
		performerSceneCounts: map[int]int{},
	}
	if len(scenes) == 0 {
		return cache, nil
	}

	sceneIDs := make([]int, 0, len(scenes))
	for _, scene := range scenes {
		sceneIDs = append(sceneIDs, scene.ID)
	}
	sceneFileIDs, err := s.repo.Scene.GetManyFileIDs(ctx, sceneIDs)
	if err != nil {
		return nil, err
	}

	fileIDs := collectPrimaryFileIDs(scenes, sceneFileIDs)
	if len(fileIDs) == 0 {
		return cache, nil
	}
	files, err := s.repo.File.Find(ctx, fileIDs...)
	if err != nil {
		return nil, err
	}

	filesByID := map[models.FileID]*models.VideoFile{}
	for _, file := range files {
		video, ok := file.(*models.VideoFile)
		if !ok {
			continue
		}
		filesByID[video.ID] = video
	}
	for index, scene := range scenes {
		if index >= len(sceneFileIDs) {
			break
		}
		fileID, ok := primaryFileID(scene, sceneFileIDs[index])
		if !ok {
			continue
		}
		if file := filesByID[fileID]; file != nil {
			cache.primaryFiles[scene.ID] = file
		}
	}

	return cache, nil
}

func collectPrimaryFileIDs(scenes []*models.Scene, sceneFileIDs [][]models.FileID) []models.FileID {
	seen := map[models.FileID]struct{}{}
	var ret []models.FileID
	for index, scene := range scenes {
		if index >= len(sceneFileIDs) {
			break
		}
		fileID, ok := primaryFileID(scene, sceneFileIDs[index])
		if !ok {
			continue
		}
		if _, ok := seen[fileID]; ok {
			continue
		}
		seen[fileID] = struct{}{}
		ret = append(ret, fileID)
	}
	return ret
}

func primaryFileID(scene *models.Scene, fileIDs []models.FileID) (models.FileID, bool) {
	if scene.PrimaryFileID != nil {
		return *scene.PrimaryFileID, true
	}
	if len(fileIDs) > 0 {
		return fileIDs[0], true
	}
	return 0, false
}

func (c *sceneItemCache) performerSceneCount(ctx context.Context, repo models.Repository, performerID int) (int, error) {
	if count, ok := c.performerSceneCounts[performerID]; ok {
		return count, nil
	}
	count, err := repo.Scene.CountByPerformerID(ctx, performerID)
	if err != nil {
		return 0, err
	}
	c.performerSceneCounts[performerID] = count
	return count, nil
}

func (s *Service) sceneItem(ctx context.Context, scene *models.Scene, cache *sceneItemCache) (SceneItem, error) {
	item := SceneItem{
		ID:         scene.ID,
		Title:      scene.GetTitle(),
		Path:       scene.Path,
		ResumeTime: scene.ResumeTime,
		Rating:     scene.Rating,
		Organized:  scene.Organized,
	}
	if scene.Date != nil {
		item.Date = scene.Date.String()
	}

	if primary := cache.primaryFiles[scene.ID]; primary != nil {
		scene.Files.Set([]*models.VideoFile{primary})
		item.Path = primary.Path
		item.Duration = primary.Duration
	} else {
		if err := scene.LoadPrimaryFile(ctx, s.repo.File); err != nil {
			return SceneItem{}, err
		}
		if primary := scene.Files.Primary(); primary != nil {
			item.Path = primary.Path
			item.Duration = primary.Duration
		}
	}

	studio, err := s.repo.Studio.FindBySceneID(ctx, scene.ID)
	if err != nil {
		return SceneItem{}, err
	}
	if studio != nil {
		item.Studio = studio.Name
	}

	performers, err := s.repo.Performer.FindBySceneID(ctx, scene.ID)
	if err != nil {
		return SceneItem{}, err
	}
	for _, performer := range performers {
		sceneCount, err := cache.performerSceneCount(ctx, s.repo, performer.ID)
		if err != nil {
			return SceneItem{}, err
		}
		item.Performers = append(item.Performers, PerformerItem{
			ID:          performer.ID,
			Name:        performer.Name,
			Rating:      performer.Rating,
			BirthYear:   modelDateYear(performer.Birthdate),
			HeightCm:    performer.Height,
			CareerStart: modelDateYear(performer.CareerStart),
			CareerEnd:   modelDateYear(performer.CareerEnd),
			SceneCount:  &sceneCount,
		})
	}

	tags, err := s.repo.Tag.FindBySceneID(ctx, scene.ID)
	if err != nil {
		return SceneItem{}, err
	}
	for _, tag := range tags {
		item.Tags = append(item.Tags, tag.Name)
	}

	return item, nil
}

func modelDateYear(date *models.Date) *int {
	if date == nil {
		return nil
	}
	year := date.Year()
	return &year
}
