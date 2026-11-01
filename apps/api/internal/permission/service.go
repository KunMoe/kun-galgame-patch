package permission

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	patchModel "kun-galgame-patch-api/internal/patch/model"
	"kun-galgame-patch-api/pkg/perm"
	"kun-galgame-patch-api/pkg/userclient"

	"gorm.io/gorm"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrStale        = errors.New("permissions changed since they were read")
)

type ValidationError struct {
	Violations []string
}

func (e *ValidationError) Error() string { return strings.Join(e.Violations, "；") }

type Caller struct {
	ID    int
	Roles []string
}

func (c Caller) rank() int { return perm.Rank(c.Roles) }

func (c Caller) holds(p perm.Permission) bool { return perm.CanUser(c.ID, c.Roles, p) }

type RoleLayer struct {
	Role      string            `json:"role"`
	Baseline  []perm.Permission `json:"baseline"`
	Overrides []perm.Override   `json:"overrides"`
	Effective []perm.Permission `json:"effective"`
	Locked    bool              `json:"locked"`
	CanEdit   bool              `json:"can_edit"`
}

type Matrix struct {
	Version int               `json:"version"`
	Groups  []perm.GroupEntry `json:"groups"`
	Catalog []perm.Entry      `json:"catalog"`
	Roles   []RoleLayer       `json:"roles"`
}

type RoleChange struct {
	Role      string          `json:"role"`
	Overrides []perm.Override `json:"overrides"`
}

type UserView struct {
	Version   int                   `json:"version"`
	User      *patchModel.PatchUser `json:"user"`
	Roles     []string              `json:"roles"`
	Baseline  []perm.Permission     `json:"baseline"`
	Overrides []perm.Override       `json:"overrides"`
	Effective []perm.Permission     `json:"effective"`
	Locked    bool                  `json:"locked"`
	CanEdit   bool                  `json:"can_edit"`
}

type OverriddenUser struct {
	User      *patchModel.PatchUser `json:"user"`
	Overrides []perm.Override       `json:"overrides"`
}

type ChangeView struct {
	ID         int                   `json:"id"`
	Actor      *patchModel.PatchUser `json:"actor"`
	TargetRole *string               `json:"target_role"`
	TargetUser *patchModel.PatchUser `json:"target_user"`
	Before     []perm.Override       `json:"before"`
	After      []perm.Override       `json:"after"`
	Reason     string                `json:"reason"`
	Created    time.Time             `json:"created"`
}

type Service struct {
	store *store
	users *userclient.Client
}

func New(db *gorm.DB, users *userclient.Client) *Service {
	return &Service{store: &store{db: db}, users: users}
}

func (s *Service) Load(ctx context.Context) error {
	roles, err := s.store.roleOverrides(ctx, s.store.db)
	if err != nil {
		return err
	}
	users, err := s.store.userOverrides(ctx, s.store.db, 0)
	if err != nil {
		return err
	}
	perm.Load(roles, users)
	return nil
}

func (s *Service) StartRefresher(interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := s.Load(context.Background()); err != nil {
					slog.Warn("权限覆盖刷新失败，继续沿用上一次的有效权限", "error", err)
				}
			}
		}
	}()
	return func() { close(done) }
}

func (s *Service) reload(ctx context.Context) {
	if err := s.Load(ctx); err != nil {
		slog.Warn("权限写入后重载失败，等待定时刷新收敛", "error", err)
	}
}

func (s *Service) Matrix(ctx context.Context, caller Caller) (Matrix, error) {
	stored, err := s.store.roleOverrides(ctx, s.store.db)
	if err != nil {
		return Matrix{}, err
	}
	version, err := s.store.roleVersion(ctx, s.store.db)
	if err != nil {
		return Matrix{}, err
	}
	layers := make([]RoleLayer, 0, len(perm.Roles))
	for _, role := range perm.Roles {
		locked := role == perm.RoleRen
		overrides := stored[role]
		if locked {
			overrides = nil
		}
		layers = append(layers, RoleLayer{
			Role:      role,
			Baseline:  perm.Baseline(role),
			Overrides: nonNil(overrides),
			Effective: perm.Apply(perm.Baseline(role), overrides),
			Locked:    locked,
			CanEdit:   !locked && perm.RoleRank(role) < caller.rank(),
		})
	}
	return Matrix{Version: version, Groups: perm.Groups(), Catalog: perm.Catalog(), Roles: layers}, nil
}

func (s *Service) ApplyRoles(ctx context.Context, caller Caller, version int, reason string, changes []RoleChange) (Matrix, error) {
	err := s.store.lockedTx(ctx, func(tx *gorm.DB) error {
		current, err := s.store.roleVersion(ctx, tx)
		if err != nil {
			return err
		}
		if current != version {
			return ErrStale
		}
		stored, err := s.store.roleOverrides(ctx, tx)
		if err != nil {
			return err
		}
		if vs := validateRoleChanges(caller, stored, changes); len(vs) > 0 {
			return &ValidationError{Violations: vs}
		}
		written := 0
		for _, ch := range changes {
			after := perm.Canonical(ch.Overrides)
			if slices.Equal(after, perm.Canonical(stored[ch.Role])) {
				continue
			}
			if err := replaceRole(tx, caller.ID, reason, ch.Role, stored[ch.Role], after); err != nil {
				return err
			}
			written++
		}
		if written == 0 {
			return &ValidationError{Violations: []string{"没有需要保存的调整"}}
		}
		return nil
	})
	if err != nil {
		return Matrix{}, err
	}
	s.reload(ctx)
	return s.Matrix(ctx, caller)
}

func validateRoleChanges(caller Caller, stored map[string][]perm.Override, changes []RoleChange) []string {
	var vs []string
	next := map[string][]perm.Override{}
	for r, ovs := range stored {
		next[r] = ovs
	}
	seen := map[string]bool{}
	for _, ch := range changes {
		switch {
		case !perm.IsRole(ch.Role):
			vs = append(vs, fmt.Sprintf("未知角色 %q", ch.Role))
			continue
		case ch.Role == perm.RoleRen:
			vs = append(vs, "莲（ren）恒持全部权限，不可调整")
			continue
		case seen[ch.Role]:
			vs = append(vs, fmt.Sprintf("角色 %s 重复出现", ch.Role))
			continue
		case perm.RoleRank(ch.Role) >= caller.rank():
			vs = append(vs, fmt.Sprintf("只能调整低于自己的角色，%s 不在此列", ch.Role))
			continue
		}
		seen[ch.Role] = true
		vs = append(vs, validateOverrides(caller, perm.Baseline(ch.Role), stored[ch.Role], ch.Overrides, ch.Role)...)
		next[ch.Role] = ch.Overrides
	}
	if len(vs) > 0 {
		return vs
	}
	admin := perm.Apply(perm.Baseline("admin"), next["admin"])
	for _, p := range perm.Apply(perm.Baseline("moderator"), next["moderator"]) {
		if !slices.Contains(admin, p) {
			vs = append(vs, fmt.Sprintf("版主不能持有管理员没有的权限：%s", p))
		}
	}
	return vs
}

// An override left as stored passes even for a key the caller lacks; otherwise
// one override set by a higher rank would block every save on that target.
func validateOverrides(caller Caller, baseline []perm.Permission, stored, overrides []perm.Override, target string) []string {
	var vs []string
	seen := map[perm.Permission]bool{}
	for _, ov := range overrides {
		inBase := slices.Contains(baseline, ov.Permission)
		switch {
		case !perm.IsKnown(ov.Permission):
			vs = append(vs, fmt.Sprintf("%s：未知权限 %q", target, ov.Permission))
		case seen[ov.Permission]:
			vs = append(vs, fmt.Sprintf("%s：权限 %s 重复出现", target, ov.Permission))
		case ov.Effect != perm.EffectGrant && ov.Effect != perm.EffectRevoke:
			vs = append(vs, fmt.Sprintf("%s：未知效果 %q", target, ov.Effect))
		case ov.Effect == perm.EffectGrant && inBase, ov.Effect == perm.EffectRevoke && !inBase:
			vs = append(vs, fmt.Sprintf("%s：%s 的覆盖与基线相同，没有改变任何东西", target, ov.Permission))
		case !slices.Contains(stored, ov) && !caller.holds(ov.Permission):
			vs = append(vs, fmt.Sprintf("%s：你自己没有 %s，不能授予或撤销它", target, ov.Permission))
		}
		seen[ov.Permission] = true
	}
	for _, ov := range stored {
		if !seen[ov.Permission] && !caller.holds(ov.Permission) {
			vs = append(vs, fmt.Sprintf("%s：你自己没有 %s，不能移除它的覆盖", target, ov.Permission))
		}
	}
	return vs
}

// targetRoles reads the account service directly: a role the user lost ten
// minutes ago must not decide whether the caller outranks them.
func (s *Service) targetRoles(ctx context.Context, uid int) (*userclient.Brief, []string, error) {
	s.users.Invalidate(uint(uid))
	b, err := s.users.User(ctx, uint(uid))
	if err != nil {
		return nil, nil, err
	}
	if b == nil {
		return nil, nil, ErrUserNotFound
	}
	return b, rankedRoles(b), nil
}

func rankedRoles(b *userclient.Brief) []string {
	roles := []string{}
	for _, r := range perm.Roles {
		if slices.Contains(b.Roles, r) || slices.Contains(b.SiteRoles, r) {
			roles = append(roles, r)
		}
	}
	return roles
}

func (s *Service) User(ctx context.Context, caller Caller, uid int) (UserView, error) {
	b, roles, err := s.targetRoles(ctx, uid)
	if err != nil {
		return UserView{}, err
	}
	stored, err := s.store.userOverrides(ctx, s.store.db, uid)
	if err != nil {
		return UserView{}, err
	}
	version, err := s.store.userVersion(ctx, s.store.db, uid)
	if err != nil {
		return UserView{}, err
	}
	return userView(caller, version, b, roles, stored[uid]), nil
}

func userView(caller Caller, version int, b *userclient.Brief, roles []string, overrides []perm.Override) UserView {
	locked := slices.Contains(roles, perm.RoleRen)
	return UserView{
		Version:   version,
		User:      patchModel.NewPatchUser(b),
		Roles:     roles,
		Baseline:  perm.RolesBaseline(roles),
		Overrides: nonNil(overrides),
		Effective: perm.Effective(int(b.ID), roles),
		Locked:    locked,
		CanEdit:   !locked && perm.Rank(roles) < caller.rank(),
	}
}

func (s *Service) ReplaceUser(ctx context.Context, caller Caller, uid, version int, reason string, overrides []perm.Override) (UserView, error) {
	b, roles, err := s.targetRoles(ctx, uid)
	if err != nil {
		return UserView{}, err
	}
	if slices.Contains(roles, perm.RoleRen) || perm.Rank(roles) >= caller.rank() {
		return UserView{}, &ValidationError{Violations: []string{"只能调整角色低于自己的用户"}}
	}
	after := perm.Canonical(overrides)
	var newVersion int
	err = s.store.lockedTx(ctx, func(tx *gorm.DB) error {
		current, err := s.store.userVersion(ctx, tx, uid)
		if err != nil {
			return err
		}
		if current != version {
			return ErrStale
		}
		stored, err := s.store.userOverrides(ctx, tx, uid)
		if err != nil {
			return err
		}
		label := b.Name
		if label == "" {
			label = fmt.Sprintf("用户 #%d", uid)
		}
		if vs := validateOverrides(caller, perm.RolesBaseline(roles), stored[uid], after, label); len(vs) > 0 {
			return &ValidationError{Violations: vs}
		}
		if slices.Equal(after, perm.Canonical(stored[uid])) {
			return &ValidationError{Violations: []string{"没有需要保存的调整"}}
		}
		if err := replaceUser(tx, caller.ID, reason, uid, stored[uid], after); err != nil {
			return err
		}
		newVersion, err = s.store.userVersion(ctx, tx, uid)
		return err
	})
	if err != nil {
		return UserView{}, err
	}
	s.reload(ctx)
	return userView(caller, newVersion, b, roles, after), nil
}

func (s *Service) Overridden(ctx context.Context) ([]OverriddenUser, error) {
	stored, err := s.store.userOverrides(ctx, s.store.db, 0)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(stored))
	for uid := range stored {
		ids = append(ids, uint(uid))
	}
	slices.Sort(ids)
	named := s.namer(ctx, ids)
	out := make([]OverriddenUser, 0, len(ids))
	for _, id := range ids {
		out = append(out, OverriddenUser{User: named(int(id)), Overrides: perm.Canonical(stored[int(id)])})
	}
	return out, nil
}

func (s *Service) Changes(ctx context.Context, f ChangeFilter, page, limit int) ([]ChangeView, int64, error) {
	rows, total, err := s.store.changes(ctx, f, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint, 0, len(rows)*2)
	for _, r := range rows {
		ids = append(ids, uint(r.ActorID))
		if r.TargetUserID != nil {
			ids = append(ids, uint(*r.TargetUserID))
		}
	}
	named := s.namer(ctx, ids)
	out := make([]ChangeView, len(rows))
	for i, r := range rows {
		v := ChangeView{
			ID:         r.ID,
			Actor:      named(r.ActorID),
			TargetRole: r.TargetRole,
			Before:     decode(r.Before),
			After:      decode(r.After),
			Reason:     r.Reason,
			Created:    r.Created,
		}
		if r.TargetUserID != nil {
			v.TargetUser = named(*r.TargetUserID)
		}
		out[i] = v
	}
	return out, total, nil
}

func (s *Service) namer(ctx context.Context, ids []uint) func(int) *patchModel.PatchUser {
	people, err := s.users.Users(ctx, ids)
	if err != nil {
		slog.Warn("权限管理：读取用户资料失败，按 id 展示", "error", err)
	}
	return func(id int) *patchModel.PatchUser {
		if u := patchModel.NewPatchUser(people[uint(id)]); u != nil {
			return u
		}
		return &patchModel.PatchUser{ID: id}
	}
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
