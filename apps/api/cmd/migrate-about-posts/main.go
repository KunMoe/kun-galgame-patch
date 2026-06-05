// cmd/migrate-about-posts seeds the about_post table (migration 014) from the
// legacy on-disk .mdx files. It is the one-time DATA half of "move /about into
// the DB": the SQL migration 014 creates the table, this imports the content.
//
// Idempotent — it upserts by slug — so it is safe to re-run after editing the
// .mdx sources to publish the changes. The runtime /about service reads from
// the DB; the .mdx files stay in the repo as the editable source.
//
// Run order: `migrate` (creates about_post) → `migrate-about-posts` (seeds it).
//
// Usage:
//
//	go run ./cmd/migrate-about-posts                  # reads KUN_POSTS_DIR (default ../web/posts)
//	go run ./cmd/migrate-about-posts -posts=apps/web/posts
//	go run ./cmd/migrate-about-posts -dry-run         # parse + report, no write
//
// Containerized (moyu-tools image bakes the posts at /posts):
//
//	docker run --rm --network <infra-net> --env-file docker/api.env \
//	  -e KUN_POSTS_DIR=/posts ghcr.io/kunmoe/moyu-tools migrate-about-posts
//
// DB DSN comes from KUN_DATABASE_URL (same as the server). Offline — it does
// NOT need any upstream service online.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	aboutModel "kun-galgame-patch-api/internal/about/model"
	aboutRepo "kun-galgame-patch-api/internal/about/repository"
	"kun-galgame-patch-api/internal/infrastructure/database"
	"kun-galgame-patch-api/pkg/config"
	"kun-galgame-patch-api/pkg/logger"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", false, "只解析并打印，不写库")
	postsFlag := flag.String("posts", "", "覆盖 KUN_POSTS_DIR（.mdx 根目录）")
	flag.Parse()

	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	dir := cfg.About.PostsDir
	if *postsFlag != "" {
		dir = *postsFlag
	}
	if _, err := os.Stat(dir); err != nil {
		slog.Error("posts 目录不可用", "dir", dir, "error", err)
		os.Exit(1)
	}

	posts, err := collect(dir)
	if err != nil {
		slog.Error("扫描 .mdx 失败", "dir", dir, "error", err)
		os.Exit(1)
	}
	slog.Info("扫描完成", "dir", dir, "count", len(posts))

	if *dryRun {
		for _, p := range posts {
			slog.Info("dry-run", "slug", p.Slug, "directory", p.Directory,
				"title", p.Title, "date", p.Date, "body_bytes", len(p.Content))
		}
		slog.Info("dry-run 结束，未写库", "count", len(posts))
		return
	}

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	repo := aboutRepo.New(db)

	var ok int
	for i := range posts {
		if err := repo.Upsert(&posts[i]); err != nil {
			slog.Error("写入失败", "slug", posts[i].Slug, "error", err)
			continue
		}
		ok++
	}
	slog.Info("迁移完成", "upserted", ok, "total", len(posts))
	if ok != len(posts) {
		os.Exit(1)
	}
}

// collect walks root, parses every .mdx into an AboutPost, and returns them
// sorted by slug for stable, reproducible output.
func collect(root string) ([]aboutModel.AboutPost, error) {
	var out []aboutModel.AboutPost
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".mdx") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fm, body, err := parseFrontmatter(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		slug := strings.TrimSuffix(rel, ".mdx")
		directory := ""
		if i := strings.IndexByte(slug, '/'); i > 0 {
			directory = slug[:i]
		}
		out = append(out, aboutModel.AboutPost{
			Slug:           slug,
			Directory:      directory,
			Title:          fm.Title,
			Banner:         fm.Banner,
			Description:    fm.Description,
			Date:           fm.Date,
			AuthorUID:      fm.AuthorUID,
			AuthorName:     fm.AuthorName,
			AuthorAvatar:   fm.AuthorAvatar,
			AuthorHomepage: fm.AuthorHomepage,
			Pin:            fm.Pin,
			Content:        body,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// parseFrontmatter splits the leading `---` YAML block from the markdown body.
// Mirrors the legacy about/service parser; an empty/missing block yields a
// zero-valued frontmatter and the whole input as the body.
func parseFrontmatter(data []byte) (aboutModel.PostFrontmatter, string, error) {
	const delim = "---"
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(src, delim) {
		return aboutModel.PostFrontmatter{}, src, nil
	}
	rest := strings.TrimLeft(src[len(delim):], "\n")
	end := strings.Index(rest, "\n"+delim)
	if end < 0 {
		return aboutModel.PostFrontmatter{}, "", fmt.Errorf("frontmatter is not terminated")
	}
	yamlSrc := rest[:end]
	body := strings.TrimLeft(rest[end+len("\n"+delim):], "\n")

	var fm aboutModel.PostFrontmatter
	if err := yaml.Unmarshal([]byte(yamlSrc), &fm); err != nil {
		return aboutModel.PostFrontmatter{}, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, body, nil
}
