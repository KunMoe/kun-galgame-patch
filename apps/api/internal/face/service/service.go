package service

import (
	"context"
	"errors"
	"strconv"

	"kun-galgame-patch-api/internal/face/dto"
	"kun-galgame-patch-api/internal/face/repository"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/imageclient"
	"kun-galgame-patch-api/pkg/problem"
	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

type Service struct {
	repo    *repository.Repository
	users   *userclient.Client
	img     *imageclient.Client
	art     *artifactclient.Client
	siteURL string
}

func New(
	repo *repository.Repository, users *userclient.Client, img *imageclient.Client, art *artifactclient.Client, siteURL string,
) *Service {
	return &Service{repo: repo, users: users, img: img, art: art, siteURL: siteURL}
}

func (s *Service) ListPatches(ctx context.Context, q *PatchQuery) (dto.List[dto.Patch], *problem.Problem) {
	filter := q.Filter
	// One row past the page decides whether there is a next one. Deriving it
	// from len(items) == limit instead hands out a cursor to an empty page
	// whenever the collection divides evenly.
	filter.Limit = q.Filter.Limit + 1
	rows, err := s.repo.ListPatches(filter)
	if err != nil {
		return dto.List[dto.Patch]{}, unavailable(err)
	}

	var next *string
	if !q.Filter.Batch && len(rows) > q.Filter.Limit {
		rows = rows[:q.Filter.Limit]
		next = EncodeCursor(q.Filter.Offset + q.Filter.Limit)
	}

	items, err := s.buildPatches(ctx, rows, q.Include)
	if err != nil {
		return dto.List[dto.Patch]{}, unavailable(err)
	}
	out := dto.NewList(items, next)

	if q.IncludeTotal {
		n, countErr := s.repo.CountPatches(q.Filter)
		if countErr != nil {
			return dto.List[dto.Patch]{}, unavailable(countErr)
		}
		out.Total = &n
	}
	if q.Filter.Batch {
		missing := missingAnchors(q, rows)
		out.Missing = &missing
	}
	return out, nil
}

func (s *Service) GetPatch(ctx context.Context, id int, include Includes) (*dto.Patch, *problem.Problem) {
	row, err := s.repo.GetPatch(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, problem.New(problem.CodeNotFound, "no patch with id "+strconv.Itoa(id))
		}
		return nil, unavailable(err)
	}
	items, err := s.buildPatches(ctx, []patchModel.Patch{*row}, include)
	if err != nil {
		return nil, unavailable(err)
	}
	return &items[0], nil
}

func (s *Service) ListPatchResources(
	ctx context.Context, patchID int, types []string, offset, limit int, include Includes, includeTotal bool,
) (dto.List[dto.Resource], *problem.Problem) {
	if _, err := s.repo.GetPatch(patchID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.List[dto.Resource]{}, problem.New(problem.CodeNotFound, "no patch with id "+strconv.Itoa(patchID))
		}
		return dto.List[dto.Resource]{}, unavailable(err)
	}

	rows, err := s.repo.ListResources(patchID, types, offset, limit+1)
	if err != nil {
		return dto.List[dto.Resource]{}, unavailable(err)
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		next = EncodeCursor(offset + limit)
	}

	items, err := s.buildResources(ctx, rows, include)
	if err != nil {
		return dto.List[dto.Resource]{}, unavailable(err)
	}
	out := dto.NewList(items, next)
	if includeTotal {
		n, countErr := s.repo.CountResources(patchID, types)
		if countErr != nil {
			return dto.List[dto.Resource]{}, unavailable(countErr)
		}
		out.Total = &n
	}
	return out, nil
}

func (s *Service) GetResource(ctx context.Context, id int, include Includes) (*dto.Resource, *problem.Problem) {
	row, err := s.repo.GetResource(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, problem.New(problem.CodeNotFound, "no resource with id "+strconv.Itoa(id))
		}
		return nil, unavailable(err)
	}
	items, err := s.buildResources(ctx, []patchModel.PatchResource{*row}, include)
	if err != nil {
		return nil, unavailable(err)
	}
	return &items[0], nil
}
