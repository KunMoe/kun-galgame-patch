package service

import (
	"context"
	"errors"
	"log/slog"
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
	repack, err := s.repo.GetRepack(row.ID)
	if err != nil {
		return nil, unavailable(err)
	}
	return s.resourceFile(ctx, row, repack)
}

func (s *Service) resourceFile(
	ctx context.Context, row *patchModel.PatchResource, repack *patchModel.PatchResourceRepack,
) (*dto.ResourceFile, *problem.Problem) {
	meta, err := s.art.Get(ctx, row.ArtifactUUID)
	if err != nil {
		return nil, artifactProblem(row.ID, err)
	}
	dl, err := s.art.Download(ctx, row.ArtifactUUID)
	if err != nil {
		return nil, artifactProblem(row.ID, err)
	}
	out := &dto.ResourceFile{
		Object:       "patch_resource_file",
		ResourceID:   strconv.Itoa(row.ID),
		ArtifactUUID: row.ArtifactUUID,
		DownloadURL:  dl.Url,
		Filename:     meta.Name,
		Bytes:        meta.FileSize,
		Blake3:       row.Blake3,
		Password:     row.Password,
		UpdatedAt:    stamp(row.UpdateTime),
	}
	if repack == nil {
		return out, nil
	}
	repackDL, err := s.art.Download(ctx, repack.ArtifactUUID)
	if errors.Is(err, artifactclient.ErrNotFound) {
		slog.Warn("repack row outlived its artifact", "resource_id", row.ID, "artifact_uuid", repack.ArtifactUUID)
		return out, nil
	}
	if err != nil {
		return nil, unavailable(err)
	}
	out.Repack = &dto.ResourceRepack{
		Object:             "patch_resource_repack",
		ArtifactUUID:       repack.ArtifactUUID,
		DownloadURL:        repackDL.Url,
		Blake3:             repack.Blake3,
		Bytes:              repack.Bytes,
		FileCount:          repack.FileCount,
		SourceArtifactUUID: repack.SourceArtifactUUID,
		SourceBlake3:       repack.SourceBlake3,
		CreatedAt:          stamp(repack.CreatedAt),
	}
	return out, nil
}

func artifactProblem(resourceID int, err error) *problem.Problem {
	if errors.Is(err, artifactclient.ErrNotFound) {
		return problem.New(problem.CodeNotFound, "the file of resource "+strconv.Itoa(resourceID)+" is no longer stored")
	}
	return unavailable(err)
}
