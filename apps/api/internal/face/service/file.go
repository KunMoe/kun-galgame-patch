package service

import (
	"context"
	"errors"
	"strconv"

	"kun-galgame-patch-api/internal/face/dto"
	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/artifactclient"
	"kun-galgame-patch-api/pkg/problem"

	"gorm.io/gorm"
)

func (s *Service) GetResourceFile(ctx context.Context, id int) (*dto.ResourceFile, *problem.Problem) {
	row, err := s.repo.GetResource(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, problem.New(problem.CodeNotFound, "no resource with id "+strconv.Itoa(id))
		}
		return nil, unavailable(err)
	}
	if row.ArtifactUUID == "" {
		return nil, problem.New(problem.CodeNotFound, "resource "+strconv.Itoa(id)+" is a link its publisher hosts elsewhere, not a file on moyu")
	}
	hash := row.Blake3
	if hash != "" {
		replaced, err := s.repo.HashOfReplacedFile(row.ID, hash)
		if err != nil {
			return nil, unavailable(err)
		}
		if replaced {
			hash = ""
		}
	}
	return s.resourceFile(ctx, row, hash)
}

func (s *Service) resourceFile(ctx context.Context, row *patchModel.PatchResource, hash string) (*dto.ResourceFile, *problem.Problem) {
	meta, err := s.art.Get(ctx, row.ArtifactUUID)
	if err != nil {
		return nil, artifactProblem(row.ID, err)
	}
	dl, err := s.art.Download(ctx, row.ArtifactUUID)
	if err != nil {
		return nil, artifactProblem(row.ID, err)
	}
	return &dto.ResourceFile{
		Object:       "patch_resource_file",
		ResourceID:   strconv.Itoa(row.ID),
		ArtifactUUID: row.ArtifactUUID,
		DownloadURL:  dl.Url,
		Filename:     meta.Name,
		Bytes:        meta.FileSize,
		Blake3:       hash,
		Password:     row.Password,
		UpdatedAt:    stamp(row.UpdateTime),
	}, nil
}

func artifactProblem(resourceID int, err error) *problem.Problem {
	if errors.Is(err, artifactclient.ErrNotFound) {
		return problem.New(problem.CodeNotFound, "the file of resource "+strconv.Itoa(resourceID)+" is no longer stored")
	}
	return unavailable(err)
}
