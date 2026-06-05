package service_test

import (
	"testing"

	"kun-galgame-patch-api/internal/about/model"
	"kun-galgame-patch-api/internal/about/service"
)

// fakeStore implements service.PostStore in-memory so the list/tree/detail
// logic can be tested without a database.
type fakeStore struct {
	posts []model.AboutPost
}

func (f *fakeStore) GetAll() ([]model.AboutPost, error) { return f.posts, nil }

func TestListAndGetPost(t *testing.T) {
	// Returned newest-first (the repository orders by date DESC); give distinct
	// dates so ordering is deterministic.
	store := &fakeStore{posts: []model.AboutPost{
		{Slug: "kun/moe", Directory: "kun", Title: "关于鲲", Date: "2026-05-02",
			Banner: "/b.png", Description: "描述", AuthorName: "作者", AuthorAvatar: "a.png",
			Content: "# 章节一\n\n## 子节\n"},
		{Slug: "dev/api", Directory: "dev", Title: "开发指南", Date: "2026-05-01",
			Banner: "/b.png", Description: "描述", AuthorName: "作者", AuthorAvatar: "a.png",
			Content: "## 安装\n\n## 使用\n"},
	}}

	svc := service.New(store)
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
	// kun/moe is newest (index 0) → no "next"; dev/api is older → it is "prev".
	if post.Prev == nil || post.Prev.Slug != "dev/api" {
		t.Errorf("prev = %+v", post.Prev)
	}
	if post.Next != nil {
		t.Errorf("expected no next, got %+v", post.Next)
	}

	if _, err := svc.GetPost("../escape"); err == nil {
		t.Error("expected error on path traversal slug")
	}
}
