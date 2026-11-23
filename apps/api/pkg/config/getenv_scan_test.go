package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The one-off tools under cmd/ never serve the admin console, so they are
// outside this rule; tests read the harness's TEST_DATABASE_DSN themselves.
var serviceRoots = []string{"../../internal", "../../cmd/server", "../../cmd/migrate"}

var directRead = regexp.MustCompile(`os\.(Getenv|LookupEnv)\(`)

func TestServicesReadEnvironmentThroughConfig(t *testing.T) {
	for _, root := range serviceRoots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if loc := directRead.FindIndex(src); loc != nil {
				line := 1 + strings.Count(string(src[:loc[0]]), "\n")
				t.Errorf("%s:%d reads the environment directly; read it in config.Load so the admin console sees it", path, line)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
