-- ============================================================
-- 腾讯云直播管理系统 - 数据库初始化脚本
-- MySQL 5.7+
-- 支持千万级并发，多租户
-- ============================================================

-- 创建数据库
CREATE DATABASE IF NOT EXISTS tencent_live DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE tencent_live;

-- ============================================================
-- 直播流表
-- 存储每次直播的流信息，包括推流地址和拉流地址
-- ============================================================
DROP TABLE IF EXISTS `streams`;
CREATE TABLE `streams` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `app_id` VARCHAR(32) NOT NULL DEFAULT 'default' COMMENT '多租户标识',
    `uid` BIGINT NOT NULL COMMENT '用户ID',
    `stream_id` VARCHAR(64) NOT NULL COMMENT '流ID（格式：appID_uid_timestamp）',
    `stream_name` VARCHAR(128) NOT NULL COMMENT '流名称（格式：appID_uid）',
    `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态: 0-inactive, 1-active, 2-closed',
    `duration` BIGINT NOT NULL DEFAULT 0 COMMENT '累计直播时长(秒)',
    `inactive_retry` INT NOT NULL DEFAULT 0 COMMENT 'inactive状态重试次数',
    
    -- 推流地址（5种格式）
    `push_rtmp` VARCHAR(512) DEFAULT NULL COMMENT 'RTMP推流地址',
    `push_web_rtc` VARCHAR(512) DEFAULT NULL COMMENT 'WebRTC推流地址',
    `push_srt` VARCHAR(512) DEFAULT NULL COMMENT 'SRT推流地址',
    `push_rtmp_over_srt` VARCHAR(512) DEFAULT NULL COMMENT 'RTMP over SRT推流地址',
    `push_rtmp_over_quic` VARCHAR(512) DEFAULT NULL COMMENT 'RTMP over QUIC推流地址',
    
    -- 拉流/播放地址（4种格式）
    `play_rtmp` VARCHAR(512) DEFAULT NULL COMMENT 'RTMP播放地址',
    `play_flv` VARCHAR(512) DEFAULT NULL COMMENT 'FLV播放地址（HTTP-FLV）',
    `play_hls` VARCHAR(512) DEFAULT NULL COMMENT 'HLS播放地址（M3U8）',
    `play_web_rtc` VARCHAR(512) DEFAULT NULL COMMENT 'WebRTC播放地址',
    
    -- 视频参数（从推流回调获取）
    `width` INT DEFAULT 0 COMMENT '视频宽度',
    `height` INT DEFAULT 0 COMMENT '视频高度',
    `video_codec` VARCHAR(16) DEFAULT NULL COMMENT '视频编码(H264/H265)',
    `audio_codec` VARCHAR(16) DEFAULT NULL COMMENT '音频编码(AAC/MP3)',
    `fps` INT DEFAULT 0 COMMENT '帧率',
    `bitrate` INT DEFAULT 0 COMMENT '码率(kbps)',
    
    -- 回调信息
    `user_ip` VARCHAR(64) DEFAULT NULL COMMENT '推流用户IP',
    `err_code` INT DEFAULT 0 COMMENT '断流错误码',
    `err_msg` VARCHAR(256) DEFAULT NULL COMMENT '断流错误信息',
    
    -- 时间字段
    `start_time` DATETIME DEFAULT NULL COMMENT '开播时间',
    `end_time` DATETIME DEFAULT NULL COMMENT '关播时间',
    `last_check_time` DATETIME DEFAULT NULL COMMENT '最后状态检测时间',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_stream_id` (`stream_id`),
    KEY `idx_app_uid_status` (`app_id`, `uid`, `status`),  -- 多租户查询优化
    KEY `idx_stream_name` (`stream_name`),
    KEY `idx_status` (`status`),
    KEY `idx_start_time` (`start_time`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='直播流表';

-- ============================================================
-- 每日直播时长统计表
-- 按租户、用户、日期汇总直播数据
-- ============================================================
DROP TABLE IF EXISTS `stream_daily_logs`;
CREATE TABLE `stream_daily_logs` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `app_id` VARCHAR(32) NOT NULL DEFAULT 'default' COMMENT '多租户标识',
    `uid` BIGINT NOT NULL COMMENT '用户ID',
    `date` VARCHAR(10) NOT NULL COMMENT '日期（格式：YYYY-MM-DD）',
    `duration` BIGINT NOT NULL DEFAULT 0 COMMENT '当日累计直播时长(秒)',
    `stream_count` INT NOT NULL DEFAULT 0 COMMENT '当日开播次数',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_app_uid_date` (`app_id`, `uid`, `date`),
    KEY `idx_app_id` (`app_id`),
    KEY `idx_uid` (`uid`),
    KEY `idx_date` (`date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='每日直播时长统计表';

-- ============================================================
-- 回调日志表
-- 存储所有腾讯云回调记录，用于问题排查和数据分析
-- ============================================================
DROP TABLE IF EXISTS `callback_logs`;
CREATE TABLE `callback_logs` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `event_type` INT NOT NULL COMMENT '事件类型: 0=断流, 1=推流, 100=录制, 200=截图, 等',
    `event_name` VARCHAR(32) DEFAULT NULL COMMENT '事件名称',
    `stream_id` VARCHAR(128) DEFAULT NULL COMMENT '流名称',
    `app_name` VARCHAR(64) DEFAULT NULL COMMENT '应用名称',
    `user_ip` VARCHAR(64) DEFAULT NULL COMMENT '用户IP',
    `event_time` BIGINT DEFAULT 0 COMMENT '事件时间戳',
    
    -- 断流相关
    `push_duration` BIGINT DEFAULT 0 COMMENT '推流时长(毫秒)',
    `errcode` INT DEFAULT 0 COMMENT '错误码',
    `errmsg` VARCHAR(256) DEFAULT NULL COMMENT '错误信息',
    
    -- 录制相关
    `video_id` VARCHAR(128) DEFAULT NULL COMMENT '点播文件ID',
    `video_url` TEXT DEFAULT NULL COMMENT '录制文件URL',
    `file_size` BIGINT DEFAULT 0 COMMENT '文件大小(字节)',
    `file_format` VARCHAR(16) DEFAULT NULL COMMENT '文件格式',
    `duration` BIGINT DEFAULT 0 COMMENT '录制时长(秒)',
    `task_id` VARCHAR(128) DEFAULT NULL COMMENT '任务ID',
    `status` VARCHAR(32) DEFAULT NULL COMMENT '状态',
    
    -- 截图相关
    `pic_url` TEXT DEFAULT NULL COMMENT '截图URL',
    
    -- 审核相关
    `confidence` DECIMAL(5,2) DEFAULT 0 COMMENT '置信度',
    `porn_score` DECIMAL(5,2) DEFAULT 0 COMMENT '涉黄分数',
    
    -- 异常相关
    `exception_type` INT DEFAULT 0 COMMENT '异常类型',
    `exception_msg` TEXT DEFAULT NULL COMMENT '异常信息',
    
    -- 原始数据（完整JSON，方便排查）
    `raw_data` TEXT DEFAULT NULL COMMENT '原始回调JSON数据',
    
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    
    PRIMARY KEY (`id`),
    KEY `idx_event_type` (`event_type`),
    KEY `idx_stream_id` (`stream_id`),
    KEY `idx_event_time` (`event_time`),
    KEY `idx_created_at` (`created_at`),
    KEY `idx_errcode` (`errcode`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='腾讯云回调日志表';

-- ============================================================
-- 创建索引优化查询
-- ============================================================

-- 用于快速查询某个租户的活跃流
-- SELECT * FROM streams WHERE app_id = 'xxx' AND status = 1;

-- 用于回调时快速查找流
-- SELECT * FROM streams WHERE stream_name = 'xxx' AND status = 1;

-- 按事件类型查询回调日志
-- SELECT * FROM callback_logs WHERE event_type = 0 ORDER BY created_at DESC LIMIT 100;

-- 按流名称查询回调日志
-- SELECT * FROM callback_logs WHERE stream_id = 'xxx' ORDER BY created_at DESC;

-- ============================================================
-- 事件类型说明
-- ============================================================
-- event_type = 0:   断流回调
-- event_type = 1:   推流回调
-- event_type = 100: 录制文件回调
-- event_type = 200: 截图回调
-- event_type = 317: 图片审核(鉴黄)回调
-- event_type = 318: 音频审核回调
-- event_type = 321: 推流异常回调
-- event_type = 332: 录制状态回调
-- event_type = 341: 录制异常回调

-- ============================================================
-- 多租户配置表
-- 存储每个租户的独立配置（如腾讯云账号、域名等）
-- ============================================================
DROP TABLE IF EXISTS `app_configs`;
CREATE TABLE `app_configs` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `app_id` VARCHAR(32) NOT NULL COMMENT '租户标识（唯一）',
    `app_name` VARCHAR(64) DEFAULT NULL COMMENT '租户名称',
    `app_secret` VARCHAR(64) DEFAULT NULL COMMENT '租户密钥（用于API鉴权）',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用, 1=启用',
    
    -- 腾讯云配置（可选，不填则使用全局配置）
    `tencent_secret_id` VARCHAR(128) DEFAULT NULL COMMENT '腾讯云 SecretId',
    `tencent_secret_key` VARCHAR(128) DEFAULT NULL COMMENT '腾讯云 SecretKey',
    `push_domain` VARCHAR(128) DEFAULT NULL COMMENT '推流域名',
    `play_domain` VARCHAR(128) DEFAULT NULL COMMENT '播放域名',
    `push_auth_key` VARCHAR(64) DEFAULT NULL COMMENT '推流鉴权Key',
    `play_auth_key` VARCHAR(64) DEFAULT NULL COMMENT '播放鉴权Key',
    
    -- 业务限制
    `max_streams` INT DEFAULT 0 COMMENT '最大同时开播数(0=不限)',
    `max_duration` INT DEFAULT 0 COMMENT '单次最大直播时长(秒,0=不限)',
    `max_bitrate` INT DEFAULT 0 COMMENT '最大码率(kbps,0=不限)',
    
    -- 回调配置
    `callback_url` VARCHAR(256) DEFAULT NULL COMMENT '自定义回调地址',
    `callback_key` VARCHAR(64) DEFAULT NULL COMMENT '回调签名Key',
    
    `remark` VARCHAR(256) DEFAULT NULL COMMENT '备注',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_app_id` (`app_id`),
    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='多租户配置表';

-- ============================================================
-- 禁播记录表
-- 记录被禁止推流的流/用户
-- ============================================================
DROP TABLE IF EXISTS `stream_bans`;
CREATE TABLE `stream_bans` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `app_id` VARCHAR(32) NOT NULL DEFAULT 'default' COMMENT '租户标识',
    `uid` BIGINT NOT NULL COMMENT '用户ID',
    `stream_name` VARCHAR(128) DEFAULT NULL COMMENT '被禁的流名称(NULL=禁止该用户所有流)',
    `ban_type` TINYINT NOT NULL DEFAULT 1 COMMENT '禁播类型: 1=临时, 2=永久',
    `reason` VARCHAR(256) DEFAULT NULL COMMENT '禁播原因',
    `operator` VARCHAR(64) DEFAULT NULL COMMENT '操作人',
    `expire_time` DATETIME DEFAULT NULL COMMENT '解禁时间(永久禁播为NULL)',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 0=已解禁, 1=禁播中',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`id`),
    KEY `idx_app_uid` (`app_id`, `uid`),
    KEY `idx_stream_name` (`stream_name`),
    KEY `idx_status` (`status`),
    KEY `idx_expire_time` (`expire_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='禁播记录表';

-- ============================================================
-- 操作日志表
-- 记录后台管理操作（用于审计）
-- ============================================================
DROP TABLE IF EXISTS `operation_logs`;
CREATE TABLE `operation_logs` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `app_id` VARCHAR(32) DEFAULT NULL COMMENT '租户标识',
    `operator` VARCHAR(64) NOT NULL COMMENT '操作人',
    `operator_ip` VARCHAR(64) DEFAULT NULL COMMENT '操作人IP',
    `action` VARCHAR(32) NOT NULL COMMENT '操作类型: create_stream, close_stream, ban_user, etc',
    `target_type` VARCHAR(32) DEFAULT NULL COMMENT '目标类型: stream, user, app, etc',
    `target_id` VARCHAR(128) DEFAULT NULL COMMENT '目标ID',
    `before_data` TEXT DEFAULT NULL COMMENT '操作前数据(JSON)',
    `after_data` TEXT DEFAULT NULL COMMENT '操作后数据(JSON)',
    `remark` VARCHAR(256) DEFAULT NULL COMMENT '备注',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    
    PRIMARY KEY (`id`),
    KEY `idx_app_id` (`app_id`),
    KEY `idx_operator` (`operator`),
    KEY `idx_action` (`action`),
    KEY `idx_target` (`target_type`, `target_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='操作日志表';

-- ============================================================
-- 实时统计表
-- 缓存热点统计数据（定时更新）
-- ============================================================
DROP TABLE IF EXISTS `realtime_stats`;
CREATE TABLE `realtime_stats` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `app_id` VARCHAR(32) NOT NULL DEFAULT 'default' COMMENT '租户标识',
    `stat_type` VARCHAR(32) NOT NULL COMMENT '统计类型: total_streams, active_streams, total_duration, etc',
    `stat_value` BIGINT NOT NULL DEFAULT 0 COMMENT '统计值',
    `stat_date` VARCHAR(10) DEFAULT NULL COMMENT '统计日期(可选)',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_app_type_date` (`app_id`, `stat_type`, `stat_date`),
    KEY `idx_stat_type` (`stat_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='实时统计表';

-- ============================================================
-- 示例数据（可选）
-- ============================================================
-- INSERT INTO streams (app_id, uid, stream_id, stream_name, status) 
-- VALUES ('default', 10001, 'default_10001_1699999999', 'default_10001', 1);

-- 插入默认租户配置
INSERT INTO `app_configs` (`app_id`, `app_name`, `status`, `remark`) 
VALUES ('default', '默认租户', 1, '系统默认租户，未指定app_id时使用');

-- ============================================================
-- 常用查询SQL（供Webman后台参考）
-- ============================================================

-- 1. 查询某租户当前活跃流数量
-- SELECT COUNT(*) FROM streams WHERE app_id = 'xxx' AND status = 1;

-- 2. 查询某租户今日开播次数和总时长
-- SELECT SUM(stream_count) as total_count, SUM(duration) as total_duration 
-- FROM stream_daily_logs WHERE app_id = 'xxx' AND date = '2026-05-10';

-- 3. 查询某用户的直播历史
-- SELECT * FROM streams WHERE app_id = 'xxx' AND uid = 10001 ORDER BY created_at DESC LIMIT 20;

-- 4. 查询异常断流记录
-- SELECT * FROM callback_logs WHERE event_type = 0 AND errcode > 0 ORDER BY created_at DESC LIMIT 100;

-- 5. 查询鉴黄告警
-- SELECT * FROM callback_logs WHERE event_type = 317 AND porn_score > 80 ORDER BY created_at DESC;

-- 6. 统计各租户直播数据
-- SELECT app_id, COUNT(*) as stream_count, SUM(duration) as total_duration 
-- FROM streams GROUP BY app_id;

-- 7. 查询被禁播的用户
-- SELECT * FROM stream_bans WHERE status = 1 ORDER BY created_at DESC;
