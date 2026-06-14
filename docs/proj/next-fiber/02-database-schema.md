# 数据库 Schema 与 GORM 模型映射

> **变更（2026-04-21）：Galgame 元数据外移到 Wiki Service**
>
> 以下 Prisma schema 文件的所有模型**不再在本项目中实现**，对应的 Go 模型/仓储/迁移均跳过：
>
> - `patch_char.prisma` — `patch_char`, `patch_char_alias`, `patch_char_relation`, `patch_char_person_relation`
> - `patch_person.prisma` — `patch_person`, `patch_person_alias`, `patch_person_relation`
> - `patch_release.prisma` — `patch_release`
> - `patch_media.prisma` — `patch_cover`, `patch_screenshot`
>
> 所有 Galgame 元数据（角色、声优/人物、发售信息、封面、截图）统一从 Galgame Wiki Service 通过 `GalgameClient` 获取，详见 `docs/galgame_wiki/integration-guide.md`。本项目只保留「补丁-VNDB ID」绑定关系（即 `patch.vndb_id` 字段），由前端/后端以该字段为 key 去 Wiki API 查询。
>
> 同时影响：
> - `edit/sync/` 中的 VNDB 同步逻辑（cover/screenshot/char/person/release）**全部删除**，创建补丁时不再在本地落盘这些元数据。
> - Next.js 的 `/api/character`、`/api/person`、`/api/release` 端点**不再迁移到 Go 端**，前端改为直接调 Wiki Service 或通过本服务代理转发。
> - 原 `apps/api/internal/patch/model/model.go` 中的 `PatchCover`、`PatchScreenshot` 模型**应删除**。
> - 本项目从 30 个模型缩减到 **21 个**（去掉 10 个元数据模型）。

## 现有 Prisma Schema 概览

项目现余 10 个 Prisma schema 文件，定义 21 个模型（已剔除 Wiki 化的元数据表）：

| Schema 文件 | 模型 | 说明 |
|------------|------|------|
| `user.prisma` | user, oauth_account, admin_log, user_follow_relation, user_message, user_patch_favorite_relation, user_patch_contribute_relation, user_patch_comment_like_relation, user_patch_resource_like_relation | 用户 + 关系 + 消息 |
| `patch.prisma` | patch | 补丁主表（仅保留 `vndb_id` 作为 Wiki 外键） |
| `patch_resource.prisma` | patch_resource | 补丁资源 |
| `patch_comment.prisma` | patch_comment | 补丁评论 |
| `patch_tag.prisma` | patch_tag, patch_tag_relation | 标签 + 关联 |
| `patch_company.prisma` | patch_company, patch_company_relation | 公司 + 关联 |
| `patch_alias.prisma` | patch_alias | 补丁别名 |
| `patch_link.prisma` | patch_link | 外部链接 |
| `patch_activity.prisma` | patch_activity | 活动追踪 |
| `chat.prisma` | chat_room, chat_member, chat_message, chat_message_seen, chat_message_reaction, chat_message_edit_history | 聊天系统 |

已废弃（**不要迁移到 Go**）：`patch_char.prisma`、`patch_person.prisma`、`patch_release.prisma`、`patch_media.prisma`。

## 已完成的 Schema 变更

以下变更已在 `docs/prisma/MIGRATION_NOTES.md` 中记录并已应用到 Prisma schema：

### String[] → Json @db.JsonB（8 个字段，已剔除 Wiki 化字段）

GORM 不原生支持 PostgreSQL `text[]`，统一改为 `jsonb`：

| 模型 | 字段 |
|------|------|
| patch | `type`, `language`, `engine`, `platform` |
| patch_resource | `type`, `language`, `platform` |
| patch_company | `primary_language`, `official_website`, `parent_brand`, `alias` |
| patch_tag | `alias` |

> ~~`patch_release.platforms`、`patch_release.languages`、`patch_char.roles`、`patch_person.roles`、`patch_person.links`~~ — 这些字段随整个模型一起废弃，不需要 Go 端建模。

### 反范式化计数字段（8 个字段）

消除 `_count` 子查询，提升列表页性能：

| 模型 | 字段 | 说明 |
|------|------|------|
| user | `follower_count`, `following_count` | 粉丝/关注数 |
| patch | `favorite_count`, `contribute_count`, `comment_count`, `resource_count` | 收藏/贡献/评论/资源数 |
| patch_comment | `like_count` | 评论点赞数 |
| patch_resource | `like_count` | 资源点赞数 |

### OAuth 集成

新增 `oauth_account` 表，关联 user 与 OAuth provider。

## GORM 模型映射

### user 模块

```go
// User 用户主表
type User struct {
    ID              int            `gorm:"primaryKey;autoIncrement" json:"id"`
    Name            string         `gorm:"uniqueIndex;type:varchar(17);not null" json:"name"`
    Email           string         `gorm:"uniqueIndex;type:varchar(1007);not null" json:"email"`
    Password        string         `gorm:"type:varchar(1007);not null" json:"-"`
    IP              string         `gorm:"type:varchar(233);default:''" json:"-"`
    Avatar          string         `gorm:"type:varchar(233);default:''" json:"avatar"`
    Role            int            `gorm:"default:1" json:"role"`
    Status          int            `gorm:"default:0" json:"status"`
    RegisterTime    time.Time      `gorm:"autoCreateTime" json:"register_time"`
    Moemoepoint     int            `gorm:"default:0" json:"moemoepoint"`
    Bio             string         `gorm:"type:varchar(107);default:''" json:"bio"`
    DailyImageCount int            `gorm:"default:0" json:"-"`
    DailyCheckIn    int            `gorm:"default:0" json:"-"`
    DailyUploadSize int            `gorm:"default:0" json:"-"`
    LastLoginTime   string         `gorm:"default:''" json:"-"`
    FollowerCount   int            `gorm:"default:0" json:"follower_count"`
    FollowingCount  int            `gorm:"default:0" json:"following_count"`
    Created         time.Time      `gorm:"autoCreateTime" json:"created"`
    Updated         time.Time      `gorm:"autoUpdateTime" json:"updated"`
}

// OAuthAccount OAuth 关联
type OAuthAccount struct {
    ID       int       `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID   int       `gorm:"index;not null" json:"user_id"`
    Provider string    `gorm:"type:varchar(50);default:'kun-oauth'" json:"provider"`
    Sub      string    `gorm:"uniqueIndex;type:varchar(255);not null" json:"sub"`
    Created  time.Time `gorm:"autoCreateTime" json:"created"`
    Updated  time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// AdminLog 管理日志
type AdminLog struct {
    ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Type    string    `gorm:"not null" json:"type"`
    Content string    `gorm:"type:varchar(10007)" json:"content"`
    Status  int       `gorm:"default:0" json:"status"`
    UserID  int       `gorm:"not null" json:"user_id"`
    Created time.Time `gorm:"autoCreateTime" json:"created"`
    Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// UserFollowRelation 关注关系
type UserFollowRelation struct {
    ID          int `gorm:"primaryKey;autoIncrement" json:"id"`
    FollowerID  int `gorm:"uniqueIndex:idx_follow;not null" json:"follower_id"`
    FollowingID int `gorm:"uniqueIndex:idx_follow;not null" json:"following_id"`
}

// UserMessage 用户消息
// status: 0=未读, 1=已读, 2=批准, 3=拒绝
type UserMessage struct {
    ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Type        string    `gorm:"not null" json:"type"`
    Content     string    `gorm:"type:varchar(10007)" json:"content"`
    Status      int       `gorm:"default:0" json:"status"`
    Link        string    `gorm:"type:varchar(1007);default:''" json:"link"`
    SenderID    *int      `json:"sender_id"`
    RecipientID *int      `json:"recipient_id"`
    Created     time.Time `gorm:"autoCreateTime" json:"created"`
    Updated     time.Time `gorm:"autoUpdateTime" json:"updated"`
}
```

### patch 模块

```go
// Patch 补丁主表
type Patch struct {
    ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
    NameEnUs           string    `gorm:"type:varchar(1007);default:''" json:"name_en_us"`
    NameZhCn           string    `gorm:"type:varchar(1007);default:''" json:"name_zh_cn"`
    NameJaJp           string    `gorm:"type:varchar(1007);default:''" json:"name_ja_jp"`
    VndbID             *string   `gorm:"uniqueIndex;type:varchar(107)" json:"vndb_id"`
    BID                *int      `gorm:"uniqueIndex" json:"bid"`
    Banner             string    `gorm:"type:varchar(1007);default:''" json:"banner"`
    IntroductionZhCn   string    `gorm:"type:varchar(100007);default:''" json:"introduction_zh_cn"`
    IntroductionJaJp   string    `gorm:"type:varchar(100007);default:''" json:"introduction_ja_jp"`
    IntroductionEnUs   string    `gorm:"type:varchar(100007);default:''" json:"introduction_en_us"`
    Released           string    `gorm:"type:varchar(107);default:'unknown'" json:"released"`
    ContentLimit       string    `gorm:"type:varchar(107);default:'sfw'" json:"content_limit"`
    Status             int       `gorm:"default:0" json:"status"`
    Download           int       `gorm:"default:0" json:"download"`
    View               int       `gorm:"default:0" json:"view"`
    ResourceUpdateTime time.Time `gorm:"autoCreateTime" json:"resource_update_time"`
    Type               JSONArray `gorm:"type:jsonb;default:'[]'" json:"type"`
    Language           JSONArray `gorm:"type:jsonb;default:'[]'" json:"language"`
    Engine             JSONArray `gorm:"type:jsonb;default:'[]'" json:"engine"`
    Platform           JSONArray `gorm:"type:jsonb;default:'[]'" json:"platform"`
    FavoriteCount      int       `gorm:"default:0" json:"favorite_count"`
    ContributeCount    int       `gorm:"default:0" json:"contribute_count"`
    CommentCount       int       `gorm:"default:0" json:"comment_count"`
    ResourceCount      int       `gorm:"default:0" json:"resource_count"`
    UserID             int       `gorm:"not null" json:"user_id"`
    Created            time.Time `gorm:"autoCreateTime" json:"created"`
    Updated            time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchResource 补丁资源（D10：hash → blake3，新增 s3_key）
type PatchResource struct {
    ID                    int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Storage               string    `gorm:"not null" json:"storage"`
    Name                  string    `gorm:"type:varchar(300);default:''" json:"name"`
    ModelName             string    `gorm:"type:varchar(1007);default:''" json:"model_name"`
    LocalizationGroupName string    `gorm:"type:varchar(1007);default:''" json:"localization_group_name"`
    Size                  string    `gorm:"type:varchar(107);default:''" json:"size"`
    Code                  string    `gorm:"type:varchar(1007);default:''" json:"code"`
    Password              string    `gorm:"type:varchar(1007);default:''" json:"password"`
    Note                  string    `gorm:"type:varchar(10007);default:''" json:"note"`
    Blake3                string    `gorm:"default:''" json:"blake3"`                             // 老数据的 BLAKE3；新数据恒为 ""
    S3Key                 string    `gorm:"type:varchar(2048);default:''" json:"s3_key"`          // 完整 S3 对象键（D10）
    Content               string    `gorm:"default:''" json:"content"`
    Type                  JSONArray `gorm:"type:jsonb;default:'[]'" json:"type"`
    Language              JSONArray `gorm:"type:jsonb;default:'[]'" json:"language"`
    Platform              JSONArray `gorm:"type:jsonb;default:'[]'" json:"platform"`
    Download              int       `gorm:"default:0" json:"download"`
    Status                int       `gorm:"default:0" json:"status"`
    UpdateTime            time.Time `gorm:"autoCreateTime" json:"update_time"`
    LikeCount             int       `gorm:"default:0" json:"like_count"`
    UserID                int       `gorm:"not null" json:"user_id"`
    PatchID               int       `gorm:"not null" json:"patch_id"`
    Created               time.Time `gorm:"autoCreateTime" json:"created"`
    Updated               time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchComment 补丁评论（支持嵌套回复）
type PatchComment struct {
    ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Content   string    `gorm:"type:varchar(10007);default:''" json:"content"`
    Edit      string    `gorm:"default:''" json:"edit"`
    LikeCount int       `gorm:"default:0" json:"like_count"`
    ParentID  *int      `json:"parent_id"`
    UserID    int       `gorm:"not null" json:"user_id"`
    PatchID   int       `gorm:"not null" json:"patch_id"`
    Created   time.Time `gorm:"autoCreateTime" json:"created"`
    Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`
}
```

### 关联表模型

```go
// 用户-补丁收藏
type UserPatchFavoriteRelation struct {
    ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID  int       `gorm:"uniqueIndex:idx_user_patch_fav;not null" json:"user_id"`
    PatchID int       `gorm:"uniqueIndex:idx_user_patch_fav;not null" json:"patch_id"`
    Created time.Time `gorm:"autoCreateTime" json:"created"`
    Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// 用户-补丁贡献
type UserPatchContributeRelation struct {
    ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID  int       `gorm:"uniqueIndex:idx_user_patch_contrib;not null" json:"user_id"`
    PatchID int       `gorm:"uniqueIndex:idx_user_patch_contrib;not null" json:"patch_id"`
    Created time.Time `gorm:"autoCreateTime" json:"created"`
    Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// 用户-评论点赞
type UserPatchCommentLikeRelation struct {
    ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID    int       `gorm:"uniqueIndex:idx_user_comment_like;not null" json:"user_id"`
    CommentID int       `gorm:"uniqueIndex:idx_user_comment_like;not null" json:"comment_id"`
    Created   time.Time `gorm:"autoCreateTime" json:"created"`
    Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// 用户-资源点赞
type UserPatchResourceLikeRelation struct {
    ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
    UserID     int       `gorm:"uniqueIndex:idx_user_resource_like;not null" json:"user_id"`
    ResourceID int       `gorm:"uniqueIndex:idx_user_resource_like;not null" json:"resource_id"`
    Created    time.Time `gorm:"autoCreateTime" json:"created"`
    Updated    time.Time `gorm:"autoUpdateTime" json:"updated"`
}
```

### metadata 模块

```go
// PatchTag 标签
type PatchTag struct {
    ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Name               string    `gorm:"type:varchar(107)" json:"name"`
    Provider           string    `gorm:"type:varchar(31);default:''" json:"provider"`
    NameEnUs           string    `gorm:"type:varchar(107);default:''" json:"name_en_us"`
    Introduction       string    `gorm:"type:varchar(10007);default:''" json:"introduction"`
    IntroductionZhCn   string    `gorm:"type:varchar(10007);default:''" json:"introduction_zh_cn"`
    IntroductionJaJp   string    `gorm:"type:varchar(10007);default:''" json:"introduction_ja_jp"`
    IntroductionEnUs   string    `gorm:"type:varchar(10007);default:''" json:"introduction_en_us"`
    Count              int       `gorm:"default:0" json:"count"`
    Alias              JSONArray `gorm:"type:jsonb;default:'[]'" json:"alias"`
    Category           string    `gorm:"default:'sexual'" json:"category"`
    Created            time.Time `gorm:"autoCreateTime" json:"created"`
    Updated            time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchTagRelation 标签关联（含 spoiler_level）
type PatchTagRelation struct {
    ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID      int       `gorm:"uniqueIndex:idx_patch_tag;not null" json:"patch_id"`
    TagID        int       `gorm:"uniqueIndex:idx_patch_tag;not null" json:"tag_id"`
    SpoilerLevel int      `gorm:"default:0" json:"spoiler_level"`
    Created      time.Time `gorm:"autoCreateTime" json:"created"`
    Updated      time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchChar 角色
type PatchChar struct {
    ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Image              string    `gorm:"type:varchar(1007);default:''" json:"image"`
    Gender             string    `gorm:"default:'unknown'" json:"gender"`
    Roles              JSONArray `gorm:"type:jsonb;default:'[]'" json:"roles"`
    Role               string    `gorm:"default:'side'" json:"role"`
    Birthday           string    `gorm:"default:''" json:"birthday"`
    Bust               int       `gorm:"default:0" json:"bust"`
    Waist              int       `gorm:"default:0" json:"waist"`
    Hips               int       `gorm:"default:0" json:"hips"`
    Height             int       `gorm:"default:0" json:"height"`
    Weight             int       `gorm:"default:0" json:"weight"`
    Cup                string    `gorm:"default:''" json:"cup"`
    Age                int       `gorm:"default:0" json:"age"`
    Infobox            string    `gorm:"default:''" json:"infobox"`
    VndbCharID         *string   `gorm:"uniqueIndex;type:varchar(32)" json:"vndb_char_id"`
    BangumiCharacterID *int      `gorm:"uniqueIndex" json:"bangumi_character_id"`
    NameZhCn           string    `gorm:"type:varchar(1007);default:''" json:"name_zh_cn"`
    NameJaJp           string    `gorm:"type:varchar(1007);default:''" json:"name_ja_jp"`
    NameEnUs           string    `gorm:"type:varchar(1007);default:''" json:"name_en_us"`
    DescriptionZhCn    string    `gorm:"type:varchar(100007);default:''" json:"description_zh_cn"`
    DescriptionJaJp    string    `gorm:"type:varchar(100007);default:''" json:"description_ja_jp"`
    DescriptionEnUs    string    `gorm:"type:varchar(100007);default:''" json:"description_en_us"`
    Created            time.Time `gorm:"autoCreateTime" json:"created"`
    Updated            time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchCompany 公司
type PatchCompany struct {
    ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Name               string    `gorm:"type:varchar(107)" json:"name"`
    Logo               string    `gorm:"type:varchar(1007);default:''" json:"logo"`
    Introduction       string    `gorm:"type:varchar(10007);default:''" json:"introduction"`
    IntroductionZhCn   string    `gorm:"type:varchar(10007);default:''" json:"introduction_zh_cn"`
    IntroductionJaJp   string    `gorm:"type:varchar(10007);default:''" json:"introduction_ja_jp"`
    IntroductionEnUs   string    `gorm:"type:varchar(10007);default:''" json:"introduction_en_us"`
    Count              int       `gorm:"default:0" json:"count"`
    PrimaryLanguage    JSONArray `gorm:"type:jsonb;default:'[]'" json:"primary_language"`
    OfficialWebsite    JSONArray `gorm:"type:jsonb;default:'[]'" json:"official_website"`
    ParentBrand        JSONArray `gorm:"type:jsonb;default:'[]'" json:"parent_brand"`
    Alias              JSONArray `gorm:"type:jsonb;default:'[]'" json:"alias"`
    Created            time.Time `gorm:"autoCreateTime" json:"created"`
    Updated            time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchPerson 人物
type PatchPerson struct {
    ID                int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Image             string    `gorm:"type:varchar(1007);default:''" json:"image"`
    Roles             JSONArray `gorm:"type:jsonb;default:'[]'" json:"roles"`
    Language          string    `gorm:"default:''" json:"language"`
    Links             JSONArray `gorm:"type:jsonb;default:'[]'" json:"links"`
    VndbStaffID       *string   `gorm:"uniqueIndex;type:varchar(32)" json:"vndb_staff_id"`
    BangumiPersonID   *int      `gorm:"uniqueIndex" json:"bangumi_person_id"`
    NameZhCn          string    `gorm:"type:varchar(1007);default:''" json:"name_zh_cn"`
    NameJaJp          string    `gorm:"type:varchar(1007);default:''" json:"name_ja_jp"`
    NameEnUs          string    `gorm:"type:varchar(1007);default:''" json:"name_en_us"`
    DescriptionZhCn   string    `gorm:"type:varchar(100007);default:''" json:"description_zh_cn"`
    DescriptionJaJp   string    `gorm:"type:varchar(100007);default:''" json:"description_ja_jp"`
    DescriptionEnUs   string    `gorm:"type:varchar(100007);default:''" json:"description_en_us"`
    Birthday          string    `gorm:"default:''" json:"birthday"`
    BloodType         string    `gorm:"default:''" json:"blood_type"`
    ReferenceSource   string    `gorm:"default:''" json:"reference_source"`
    Birthplace        string    `gorm:"default:''" json:"birthplace"`
    Office            string    `gorm:"default:''" json:"office"`
    X                 string    `gorm:"default:''" json:"x"`
    Spouse            string    `gorm:"default:''" json:"spouse"`
    OfficialWebsite   string    `gorm:"default:''" json:"official_website"`
    Blog              string    `gorm:"default:''" json:"blog"`
    Created           time.Time `gorm:"autoCreateTime" json:"created"`
    Updated           time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchRelease VNDB 发售信息
type PatchRelease struct {
    ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID   int       `gorm:"index;not null" json:"patch_id"`
    RID       string    `gorm:"uniqueIndex;type:varchar(16)" json:"rid"`
    Title     string    `gorm:"type:varchar(1007)" json:"title"`
    Released  string    `gorm:"type:varchar(107);default:'2019-10-07'" json:"released"`
    Platforms JSONArray `gorm:"type:jsonb;default:'[]'" json:"platforms"`
    Languages JSONArray `gorm:"type:jsonb;default:'[]'" json:"languages"`
    Minage    int       `gorm:"default:0" json:"minage"`
    Created   time.Time `gorm:"autoCreateTime" json:"created"`
    Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`
}
```

### chat 模块

```go
// ChatRoom 聊天室
type ChatRoom struct {
    ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Name            string    `gorm:"type:varchar(107)" json:"name"`
    Link            string    `gorm:"uniqueIndex;type:varchar(17)" json:"link"`
    Avatar          string    `gorm:"type:varchar(1007)" json:"avatar"`
    Type            string    `gorm:"default:'PRIVATE'" json:"type"` // PRIVATE / GROUP
    LastMessageTime time.Time `gorm:"autoCreateTime" json:"last_message_time"`
    Created         time.Time `gorm:"autoCreateTime" json:"created"`
    Updated         time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// ChatMember 聊天成员
type ChatMember struct {
    ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Role       string    `gorm:"default:'MEMBER'" json:"role"` // OWNER / ADMIN / MEMBER
    UserID     int       `gorm:"uniqueIndex:idx_user_room;not null" json:"user_id"`
    ChatRoomID int       `gorm:"uniqueIndex:idx_user_room;not null" json:"chat_room_id"`
    Created    time.Time `gorm:"autoCreateTime" json:"created"`
    Updated    time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
    ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
    Content     string     `gorm:"type:varchar(2000);default:''" json:"content"`
    FileURL     string     `gorm:"type:varchar(1007);default:''" json:"file_url"`
    Status      string     `gorm:"default:'SENT'" json:"status"` // SENT / EDITED / DELETED
    DeletedAt   *time.Time `json:"deleted_at"`
    DeletedByID *int       `json:"deleted_by_id"`
    ChatRoomID  int        `gorm:"index;not null" json:"chat_room_id"`
    SenderID    int        `gorm:"not null" json:"sender_id"`
    ReplyToID   *int       `json:"reply_to_id"`
    Created     time.Time  `gorm:"autoCreateTime" json:"created"`
    Updated     time.Time  `gorm:"autoUpdateTime" json:"updated"`
}

// ChatMessageSeen 消息已读状态
type ChatMessageSeen struct {
    ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
    ChatMessageID int       `gorm:"uniqueIndex:idx_user_msg_seen;not null" json:"chat_message_id"`
    UserID        int       `gorm:"uniqueIndex:idx_user_msg_seen;not null" json:"user_id"`
    ReadAt        time.Time `gorm:"autoCreateTime" json:"read_at"`
}

// ChatMessageReaction 消息表情回应
type ChatMessageReaction struct {
    ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Emoji         string    `gorm:"type:varchar(10)" json:"emoji"`
    ChatMessageID int       `gorm:"uniqueIndex:idx_user_msg_emoji;not null" json:"chat_message_id"`
    UserID        int       `gorm:"uniqueIndex:idx_user_msg_emoji;not null" json:"user_id"`
    Created       time.Time `gorm:"autoCreateTime" json:"created"`
    Updated       time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// ChatMessageEditHistory 消息编辑历史
type ChatMessageEditHistory struct {
    ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PreviousContent string    `gorm:"type:varchar(2000)" json:"previous_content"`
    ChatMessageID   int       `gorm:"index;not null" json:"chat_message_id"`
    EditedAt        time.Time `gorm:"autoCreateTime" json:"edited_at"`
}
```

### 辅助模型

```go
// PatchAlias 补丁别名
type PatchAlias struct {
    ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Name    string    `gorm:"type:varchar(1007);index" json:"name"`
    PatchID int       `gorm:"index;not null" json:"patch_id"`
    Created time.Time `gorm:"autoCreateTime" json:"created"`
    Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchLink 外部链接
type PatchLink struct {
    ID      int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID int       `gorm:"uniqueIndex:idx_patch_link;index;not null" json:"patch_id"`
    Name    string    `gorm:"uniqueIndex:idx_patch_link;type:varchar(233)" json:"name"`
    URL     string    `gorm:"type:varchar(1007)" json:"url"`
    Created time.Time `gorm:"autoCreateTime" json:"created"`
    Updated time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchCover 补丁封面（VNDB）
type PatchCover struct {
    ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID      int       `gorm:"uniqueIndex:idx_patch_cover;index;not null" json:"patch_id"`
    ImageID      string    `gorm:"uniqueIndex:idx_patch_cover;type:varchar(107);index" json:"image_id"`
    URL          string    `gorm:"type:varchar(1007)" json:"url"`
    Width        int       `json:"width"`
    Height       int       `json:"height"`
    Sexual       float64   `json:"sexual"`
    Violence     float64   `json:"violence"`
    Votecount    int       `json:"votecount"`
    ThumbnailURL string    `gorm:"type:varchar(1007)" json:"thumbnail_url"`
    ThumbWidth   int       `json:"thumb_width"`
    ThumbHeight  int       `json:"thumb_height"`
    Created      time.Time `gorm:"autoCreateTime" json:"created"`
    Updated      time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// PatchScreenshot 补丁截图（VNDB）
type PatchScreenshot struct {
    ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID      int       `gorm:"uniqueIndex:idx_patch_screenshot;index;not null" json:"patch_id"`
    ImageID      string    `gorm:"uniqueIndex:idx_patch_screenshot;type:varchar(107);index" json:"image_id"`
    URL          string    `gorm:"type:varchar(1007)" json:"url"`
    Width        int       `json:"width"`
    Height       int       `json:"height"`
    Sexual       float64   `json:"sexual"`
    Violence     float64   `json:"violence"`
    Votecount    int       `json:"votecount"`
    ThumbnailURL string    `gorm:"type:varchar(1007)" json:"thumbnail_url"`
    ThumbWidth   int       `json:"thumb_width"`
    ThumbHeight  int       `json:"thumb_height"`
    OrderNo      int       `gorm:"default:0" json:"order_no"`
    Created      time.Time `gorm:"autoCreateTime" json:"created"`
    Updated      time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// 别名表（char + person 各有一套）
type PatchCharAlias struct {
    ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Name        string    `gorm:"type:varchar(233);uniqueIndex:idx_char_alias;index" json:"name"`
    PatchCharID int       `gorm:"uniqueIndex:idx_char_alias;index;not null" json:"patch_char_id"`
    Created     time.Time `gorm:"autoCreateTime" json:"created"`
    Updated     time.Time `gorm:"autoUpdateTime" json:"updated"`
}

type PatchPersonAlias struct {
    ID       int       `gorm:"primaryKey;autoIncrement" json:"id"`
    Name     string    `gorm:"type:varchar(233);uniqueIndex:idx_person_alias;index" json:"name"`
    PersonID int       `gorm:"uniqueIndex:idx_person_alias;index;not null" json:"person_id"`
    Created  time.Time `gorm:"autoCreateTime" json:"created"`
    Updated  time.Time `gorm:"autoUpdateTime" json:"updated"`
}

// 关联表
type PatchCharRelation struct {
    ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID     int       `gorm:"uniqueIndex:idx_patch_char;index;not null" json:"patch_id"`
    PatchCharID int       `gorm:"uniqueIndex:idx_patch_char;index;not null" json:"patch_char_id"`
    Created     time.Time `gorm:"autoCreateTime" json:"created"`
    Updated     time.Time `gorm:"autoUpdateTime" json:"updated"`
}

type PatchCharPersonRelation struct {
    ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchCharID   int       `gorm:"uniqueIndex:idx_char_person;index;not null" json:"patch_char_id"`
    PatchPersonID int       `gorm:"uniqueIndex:idx_char_person;index;not null" json:"patch_person_id"`
    Relation      string    `gorm:"uniqueIndex:idx_char_person;default:''" json:"relation"`
    Created       time.Time `gorm:"autoCreateTime" json:"created"`
    Updated       time.Time `gorm:"autoUpdateTime" json:"updated"`
}

type PatchCompanyRelation struct {
    ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID   int       `gorm:"uniqueIndex:idx_patch_company;not null" json:"patch_id"`
    CompanyID int       `gorm:"uniqueIndex:idx_patch_company;not null" json:"company_id"`
    Created   time.Time `gorm:"autoCreateTime" json:"created"`
    Updated   time.Time `gorm:"autoUpdateTime" json:"updated"`
}

type PatchPersonRelation struct {
    ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
    PatchID       int       `gorm:"uniqueIndex:idx_patch_person;index;not null" json:"patch_id"`
    PatchPersonID int       `gorm:"uniqueIndex:idx_patch_person;index;not null" json:"patch_person_id"`
    Created       time.Time `gorm:"autoCreateTime" json:"created"`
    Updated       time.Time `gorm:"autoUpdateTime" json:"updated"`
}
```

## JSONArray 自定义类型

jsonb 数组字段需要自定义 GORM 类型：

```go
package model

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
)

// JSONArray 用于 PostgreSQL jsonb 数组字段
type JSONArray []string

func (j *JSONArray) Scan(value interface{}) error {
    bytes, ok := value.([]byte)
    if !ok {
        return fmt.Errorf("failed to unmarshal JSONArray: %v", value)
    }
    return json.Unmarshal(bytes, j)
}

func (j JSONArray) Value() (driver.Value, error) {
    if j == nil {
        return "[]", nil
    }
    return json.Marshal(j)
}
```

## GORM 配置要点

```go
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    SkipDefaultTransaction: true,  // 避免不必要的事务开销
    NamingStrategy: schema.NamingStrategy{
        SingularTable: true,       // 表名不加复数（user 而非 users）
    },
})
```

**不使用 AutoMigrate**：所有 schema 变更通过 `cmd/migrate/` 中的 SQL 迁移文件管理，避免与 Prisma 产生冲突。
