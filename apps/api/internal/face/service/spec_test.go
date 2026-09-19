package service

import (
	"os"
	"slices"
	"strings"
	"testing"

	"kun-galgame-patch-api/internal/face/repository"

	"gopkg.in/yaml.v3"
)

const specPath = "../../../../../docs/open-api/moyu-openapi.yaml"

type specParam struct {
	Ref    string `yaml:"$ref"`
	Name   string `yaml:"name"`
	Schema struct {
		Enum  []string `yaml:"enum"`
		Items struct {
			Ref  string   `yaml:"$ref"`
			Enum []string `yaml:"enum"`
		} `yaml:"items"`
	} `yaml:"schema"`
}

type specDoc struct {
	Paths map[string]struct {
		Get struct {
			OperationID string      `yaml:"operationId"`
			Parameters  []specParam `yaml:"parameters"`
		} `yaml:"get"`
	} `yaml:"paths"`
	Components struct {
		Parameters map[string]specParam `yaml:"parameters"`
		Schemas    map[string]struct {
			Enum       []string `yaml:"enum"`
			Properties map[string]struct {
				Enum []string `yaml:"enum"`
			} `yaml:"properties"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

func loadSpec(t *testing.T) (*specDoc, map[string]map[string]specParam) {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var doc specDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	ops := map[string]map[string]specParam{}
	for _, item := range doc.Paths {
		params := map[string]specParam{}
		for _, p := range item.Get.Parameters {
			if p.Ref != "" {
				p = doc.Components.Parameters[strings.TrimPrefix(p.Ref, "#/components/parameters/")]
			}
			params[p.Name] = p
		}
		ops[item.Get.OperationID] = params
	}
	return &doc, ops
}

func TestSpecVocabulariesMatchParser(t *testing.T) {
	doc, ops := loadSpec(t)

	vocabularies := map[string][]string{
		"PatchType":     PatchTypes,
		"PatchLanguage": PatchLanguages,
		"PatchPlatform": PatchPlatforms,
	}
	for name, want := range vocabularies {
		if got := doc.Components.Schemas[name].Enum; !slices.Equal(got, want) {
			t.Errorf("schema %s enum = %v, parser accepts %v", name, got, want)
		}
	}
	for param, schema := range map[string]string{"type": "PatchType", "language": "PatchLanguage", "platform": "PatchPlatform"} {
		if got := ops["listPatches"][param].Schema.Items.Ref; got != "#/components/schemas/"+schema {
			t.Errorf("listPatches %s items = %q, want the %s schema", param, got, schema)
		}
	}

	if got := ops["listPatches"]["sort"].Schema.Enum; !slices.Equal(got, repository.SortKeys) {
		t.Errorf("sort enum = %v, parser accepts %v", got, repository.SortKeys)
	}

	includes := map[string][]string{
		"listPatches":        PatchIncludes,
		"getPatch":           PatchIncludes,
		"listPatchResources": ResourceIncludes,
		"getResource":        ResourceIncludes,
	}
	for op, want := range includes {
		if got := ops[op]["include"].Schema.Items.Enum; !slices.Equal(got, want) {
			t.Errorf("%s include enum = %v, parser accepts %v", op, got, want)
		}
	}
}

func TestSpecFieldErrorParameterEnum(t *testing.T) {
	doc, ops := loadSpec(t)

	var declared []string
	for _, params := range ops {
		for name := range params {
			if !slices.Contains(declared, name) {
				declared = append(declared, name)
			}
		}
	}
	got := slices.Clone(doc.Components.Schemas["FieldError"].Properties["parameter"].Enum)
	slices.Sort(declared)
	slices.Sort(got)
	if !slices.Equal(got, declared) {
		t.Errorf("FieldError.parameter enum = %v, operations declare %v", got, declared)
	}
}
