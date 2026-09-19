package handler

import (
	"slices"
	"strings"

	face "kun-galgame-patch-api/internal/face/service"
	"kun-galgame-patch-api/internal/patch/dto"
)

func validateResourceVocab(req *dto.PatchResourceCreateRequest) string {
	return validateVocab(req.Type, req.Language, req.Platform, face.PatchTypes)
}

func validateVocab(types, langs, plats, allowedTypes []string) string {
	if msg := closedVocab("type", types, allowedTypes); msg != "" {
		return msg
	}
	if msg := closedVocab("language", langs, face.PatchLanguages); msg != "" {
		return msg
	}
	return closedVocab("platform", plats, face.PatchPlatforms)
}

func closedVocab(kind string, got, allowed []string) string {
	for _, g := range got {
		if !slices.Contains(allowed, g) {
			return "unknown " + kind + " " + g + " (accepted: " + strings.Join(allowed, ", ") + ")"
		}
	}
	return ""
}
