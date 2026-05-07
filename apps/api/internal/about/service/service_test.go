package service_test

import (
	"os"
	"path/filepath"
	"testing"

	"kun-galgame-patch-api/internal/about/service"
)

func writePost(t *testing.T, dir, slug, title, body string) {
	t.Helper()
	full := filepath.Join(dir, slug+".mdx")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "---\ntitle: " + title + "\ndate: 2026-05-01\nbanner: '/b.png'\n" +
		"description: '描述'\nauthorName: '作者'\nauthorAvatar: 'a.png'\n---\n" +
		body
	if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListAndGetPost(t *testing.T) {
	dir := t.TempDir()
	writePost(t, dir, "kun/moe", "关于鲲", "# 章节一\n\n## 子节\n")
	writePost(t, dir, "dev/api", "开发指南", "## 安装\n\n## 使用\n")

	svc := service.New(dir)
	list, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list.Items))
	}
	if list.Tree.Type != "directory" || len(list.Tree.Children) != 2 {
		t.Fatalf("tree shape unexpected: %+v", list.Tree)
	}

	post, err := svc.GetPost("kun/moe")
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if post.Frontmatter.Title != "关于鲲" {
		t.Errorf("title = %q", post.Frontmatter.Title)
	}
	if len(post.TOC) != 2 {
		t.Fatalf("expected 2 TOC items, got %d", len(post.TOC))
	}
	if post.TOC[0].ID != "章节一" || post.TOC[1].ID != "子节" {
		t.Errorf("TOC ids = %+v", post.TOC)
	}

	if _, err := svc.GetPost("../escape"); err == nil {
		t.Error("expected error on path traversal slug")
	}
}
