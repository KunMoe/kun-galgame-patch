-- 013_patch_resource_revision: 资源「按字段」编辑历史(diff)。
--
-- patch_resource 原地修改时,patch_resource_file_history 只记录「文件替换」,
-- 不记录 语言/平台/类型/备注/名称/大小 等元数据改动。本表为每次 UpdateResource
-- 存一条「改动 diff」——公开安全:只存字段标签 + 改动前/后的值;敏感的下载链接 /
-- 提取码 / 解压密码只以「已更新」标记,绝不存原文。供前端展示「改动前 → 改动后」。
--
-- CASCADE on delete: 资源删除即连带删除其修订(与 patch_resource_file_history 一致)。
CREATE TABLE IF NOT EXISTS patch_resource_revision (
    id           BIGSERIAL PRIMARY KEY,
    resource_id  INT NOT NULL REFERENCES patch_resource(id) ON DELETE CASCADE,
    action       VARCHAR(16) NOT NULL DEFAULT 'updated',   -- 'created' / 'updated'
    changes      JSONB NOT NULL DEFAULT '[]',              -- [{field,label,before,after}]
    reason       VARCHAR(500) NOT NULL DEFAULT '',
    actor_id     INT NOT NULL DEFAULT 0,
    actor_role   INT NOT NULL DEFAULT 0,                   -- 3=admin / 2=mod / 1=user / 0=unknown
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prr_resource ON patch_resource_revision(resource_id, created_at DESC);
