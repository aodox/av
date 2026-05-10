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
    KEY `idx_app_uid` (`app_id`, `uid`),
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
-- 示例数据（可选）
-- ============================================================
-- INSERT INTO streams (app_id, uid, stream_id, stream_name, status) 
-- VALUES ('default', 10001, 'default_10001_1699999999', 'default_10001', 1);
