-- ============================================================
-- 腾讯云直播管理系统 - 数据库迁移脚本
-- 版本: v1.0.0 -> v1.1.0
-- 日期: 2026-05-11
-- 说明: 完善多租户支持
-- ============================================================

USE tencent_live;

-- ============================================================
-- 1. streams 表：添加多租户优化复合索引
-- ============================================================

-- 检查并添加复合索引（用于多租户查询优化）
-- 场景：SELECT * FROM streams WHERE app_id = ? AND uid = ? AND status = ?
SET @exist := (
    SELECT COUNT(*) FROM information_schema.statistics 
    WHERE table_schema = DATABASE() 
    AND table_name = 'streams' 
    AND index_name = 'idx_app_uid_status'
);
SET @sql := IF(
    @exist = 0,
    'ALTER TABLE streams ADD INDEX idx_app_uid_status (app_id, uid, status)',
    'SELECT "Index idx_app_uid_status already exists"'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 可选：删除旧的单字段索引（已被复合索引覆盖）
-- 注意：如果有其他查询只用 app_id 或只用 uid，请保留
-- ALTER TABLE streams DROP INDEX idx_app_id;  -- 谨慎操作

-- ============================================================
-- 2. stream_daily_logs 表：确认唯一键包含 app_id
-- ============================================================

-- 检查唯一键是否正确（应该是 app_id + uid + date）
-- 初始化脚本已经是正确的：uk_app_uid_date (app_id, uid, date)
-- 此处无需修改

-- ============================================================
-- 3. 验证索引
-- ============================================================

-- 查看 streams 表索引
SELECT 
    INDEX_NAME,
    GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS columns
FROM information_schema.statistics 
WHERE table_schema = DATABASE() 
AND table_name = 'streams'
GROUP BY INDEX_NAME;

-- 查看 stream_daily_logs 表索引
SELECT 
    INDEX_NAME,
    GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS columns
FROM information_schema.statistics 
WHERE table_schema = DATABASE() 
AND table_name = 'stream_daily_logs'
GROUP BY INDEX_NAME;

-- ============================================================
-- 4. 数据迁移检查（可选）
-- ============================================================

-- 检查是否有 app_id 为空的数据
SELECT COUNT(*) as empty_app_id_count FROM streams WHERE app_id = '' OR app_id IS NULL;
SELECT COUNT(*) as empty_app_id_count FROM stream_daily_logs WHERE app_id = '' OR app_id IS NULL;

-- 如果有空的 app_id，更新为默认值
UPDATE streams SET app_id = 'default' WHERE app_id = '' OR app_id IS NULL;
UPDATE stream_daily_logs SET app_id = 'default' WHERE app_id = '' OR app_id IS NULL;

-- ============================================================
-- 迁移完成提示
-- ============================================================
SELECT 'Migration v1.1.0 completed successfully!' AS message;
