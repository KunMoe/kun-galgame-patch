package perm

import (
	"slices"
	"sync/atomic"
)

type Permission string

const (
	ResourceEditAny    Permission = "resource.edit_any"
	ResourceDeleteAny  Permission = "resource.delete_any"
	ResourceDisableAny Permission = "resource.disable_any"
	ResourceReorder    Permission = "resource.reorder"

	PatchEditAny             Permission = "patch.edit_any"
	PatchDeleteAny           Permission = "patch.delete_any"
	GalgamePublishRestricted Permission = "galgame.publish_restricted"

	CommentEditAny   Permission = "comment.edit_any"
	CommentDeleteAny Permission = "comment.delete_any"
	RatingDeleteAny  Permission = "rating.delete_any"

	ChatRoomCreate       Permission = "chat.room_create"
	ChatMessageDeleteAny Permission = "chat.message_delete_any"

	AdminAccess      Permission = "admin.access"
	TrustReview      Permission = "trust.review"
	DocManage        Permission = "doc.manage"
	SettingManage    Permission = "setting.manage"
	CatalogMergeUndo Permission = "catalog.merge_undo"
	UserPurge        Permission = "user.purge"
	PermissionManage Permission = "permission.manage"
)

type Group string

const (
	GroupResource Group = "resource"
	GroupPatch    Group = "patch"
	GroupComment  Group = "comment"
	GroupChat     Group = "chat"
	GroupAdmin    Group = "admin"
)

type GroupEntry struct {
	Key   Group  `json:"key"`
	Label string `json:"label"`
}

var groups = []GroupEntry{
	{GroupResource, "补丁资源"},
	{GroupPatch, "游戏页"},
	{GroupComment, "评论与评分"},
	{GroupChat, "聊天"},
	{GroupAdmin, "管理后台"},
}

func Groups() []GroupEntry { return slices.Clone(groups) }

type Entry struct {
	Key         Permission `json:"key"`
	Group       Group      `json:"group"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Sensitive   bool       `json:"sensitive"`
}

var catalog = []Entry{
	{ResourceEditAny, GroupResource, "编辑他人资源", "修改任何人发布的补丁资源，包括替换文件与下载链接", false},
	{ResourceDeleteAny, GroupResource, "删除他人资源", "删除任何人发布的补丁资源及其文件，不可恢复", true},
	{ResourceDisableAny, GroupResource, "禁用他人资源下载", "禁用或恢复任何资源的下载", false},
	{ResourceReorder, GroupResource, "调整资源展示顺序", "在游戏详情页固定补丁资源的展示顺序，未固定的资源随机展示", false},

	{PatchEditAny, GroupPatch, "修改他人游戏页", "修改任何人创建的游戏页的基本信息", false},
	{PatchDeleteAny, GroupPatch, "删除他人游戏页", "删除任何人创建的游戏页及其全部资源，不可恢复", true},
	{GalgamePublishRestricted, GroupPatch, "限制期间发布游戏", "网站开启「仅创作者可发布」时仍可发布新游戏", false},

	{CommentEditAny, GroupComment, "编辑他人评论", "修改任何人的评论，会通知作者并记录原因", false},
	{CommentDeleteAny, GroupComment, "删除他人评论", "删除任何人的评论，会通知作者并记录原因", true},
	{RatingDeleteAny, GroupComment, "删除他人评分", "删除任何人的游戏评分；评分在所有 NextMoe 站点同步删除，还需中央账号持有 catalog.rating.moderate", true},

	{ChatRoomCreate, GroupChat, "创建群聊", "创建新的群聊房间", false},
	{ChatMessageDeleteAny, GroupChat, "删除他人聊天消息", "删除任何人在聊天室发送的消息", false},

	{AdminAccess, GroupAdmin, "进入管理后台", "查看数据概览、管理日志、资源 / 评论 / 游戏列表、孤儿补丁与合并记录", false},
	{TrustReview, GroupAdmin, "处理举报审核", "领取并裁决举报审核队列中的条目", false},
	{DocManage, GroupAdmin, "管理文档", "创建、编辑、删除站点文档", false},
	{SettingManage, GroupAdmin, "修改网站设置", "修改全站开关，例如「仅创作者可发布」", true},
	{CatalogMergeUndo, GroupAdmin, "撤销游戏合并", "把已合并的游戏页拆回原状", true},
	{UserPurge, GroupAdmin, "清除用户", "不可恢复地删除一名用户在本站的全部内容", true},
	{PermissionManage, GroupAdmin, "管理权限", "调整低于自己的角色与用户的权限；只能授予或撤销自己持有的权限", true},
}

const (
	EffectGrant  = "grant"
	EffectRevoke = "revoke"

	RoleRen = "ren"
)

// Roles carries every role that can hold a permission, lowest rank first.
var Roles = []string{"creator", "moderator", "admin", RoleRen}

var moderatorBaseline = []Permission{
	ResourceEditAny, ResourceDeleteAny, ResourceDisableAny, ResourceReorder,
	PatchEditAny, GalgamePublishRestricted,
	CommentEditAny, CommentDeleteAny,
	ChatMessageDeleteAny,
	AdminAccess, TrustReview, DocManage,
}

var baselines = map[string][]Permission{
	"creator":   {GalgamePublishRestricted},
	"moderator": moderatorBaseline,
	"admin": append(slices.Clone(moderatorBaseline),
		PatchDeleteAny, RatingDeleteAny, ChatRoomCreate,
		SettingManage, CatalogMergeUndo, UserPurge, PermissionManage),
}

func InGroup(g Group) []Permission {
	var out []Permission
	for _, e := range catalog {
		if e.Group == g {
			out = append(out, e.Key)
		}
	}
	return out
}

type Override struct {
	Permission Permission `json:"permission"`
	Effect     string     `json:"effect"`
}

func Catalog() []Entry { return slices.Clone(catalog) }

func Keys() []Permission {
	out := make([]Permission, len(catalog))
	for i, e := range catalog {
		out[i] = e.Key
	}
	return out
}

func IsKnown(p Permission) bool {
	return slices.ContainsFunc(catalog, func(e Entry) bool { return e.Key == p })
}

func IsRole(role string) bool { return slices.Contains(Roles, role) }

func RoleRank(role string) int { return slices.Index(Roles, role) + 1 }

func Rank(roles []string) int {
	best := 0
	for _, r := range roles {
		best = max(best, RoleRank(r))
	}
	return best
}

func Baseline(role string) []Permission {
	if role == RoleRen {
		return Keys()
	}
	return ordered(toSet(baselines[role]))
}

// RolesBaseline is what a holder of these roles gets before personal overrides.
func RolesBaseline(roles []string) []Permission {
	return ordered(current.Load().rolesSet(roles))
}

// Canonical puts overrides in catalog order so two equal sets compare equal.
func Canonical(overrides []Override) []Override {
	out := make([]Override, 0, len(overrides))
	for _, e := range catalog {
		for _, ov := range overrides {
			if ov.Permission == e.Key {
				out = append(out, ov)
			}
		}
	}
	return out
}

func Apply(base []Permission, overrides []Override) []Permission {
	set := toSet(base)
	for _, ov := range overrides {
		if !IsKnown(ov.Permission) {
			continue
		}
		switch ov.Effect {
		case EffectGrant:
			set[ov.Permission] = struct{}{}
		case EffectRevoke:
			delete(set, ov.Permission)
		}
	}
	return ordered(set)
}

type state struct {
	roles map[string]map[Permission]struct{}
	users map[int][]Override
}

func (s *state) rolesSet(roles []string) map[Permission]struct{} {
	out := map[Permission]struct{}{}
	for _, r := range roles {
		for p := range s.roles[r] {
			out[p] = struct{}{}
		}
	}
	return out
}

var current atomic.Pointer[state]

func init() { Load(nil, nil) }

// Load swaps in the stored overrides; the refresher calls it, so a write on one
// instance reaches the others within one refresh interval.
func Load(roleOverrides map[string][]Override, userOverrides map[int][]Override) {
	s := &state{roles: map[string]map[Permission]struct{}{}, users: userOverrides}
	for _, r := range Roles {
		if r == RoleRen {
			s.roles[r] = toSet(Keys())
			continue
		}
		s.roles[r] = toSet(Apply(Baseline(r), roleOverrides[r]))
	}
	current.Store(s)
}

func Effective(uid int, roles []string) []Permission {
	if slices.Contains(roles, RoleRen) {
		return Keys()
	}
	s := current.Load()
	return Apply(ordered(s.rolesSet(roles)), s.users[uid])
}

func CanUser(uid int, roles []string, p Permission) bool {
	return slices.Contains(Effective(uid, roles), p)
}

func toSet(ps []Permission) map[Permission]struct{} {
	set := make(map[Permission]struct{}, len(ps))
	for _, p := range ps {
		set[p] = struct{}{}
	}
	return set
}

func ordered(set map[Permission]struct{}) []Permission {
	out := make([]Permission, 0, len(set))
	for _, e := range catalog {
		if _, ok := set[e.Key]; ok {
			out = append(out, e.Key)
		}
	}
	return out
}
