-- 推荐做法：
--   1) 首次部署：mysql 内执行本脚本，或直接执行 `nexpay migrate`
--   2) 启动服务：`nexpay`（不再触发 DDL）

CREATE DATABASE IF NOT EXISTS `nexpay` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `nexpay`;

CREATE TABLE IF NOT EXISTS `orders` (
  `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `out_trade_no`     VARCHAR(64)     NOT NULL,
  `channel`          VARCHAR(16)     NOT NULL,
  `trade_type`       VARCHAR(16)     NOT NULL,
  `subject`          VARCHAR(255)    NOT NULL,
  `description`      VARCHAR(512)    DEFAULT NULL,
  `amount`           BIGINT          NOT NULL COMMENT '金额（分）',
  `currency`         VARCHAR(8)      NOT NULL DEFAULT 'CNY',
  `status`           VARCHAR(16)     NOT NULL DEFAULT 'PENDING',
  `channel_trade_no` VARCHAR(64)     DEFAULT NULL,
  `payer_open_id`    VARCHAR(128)    DEFAULT NULL,
  `client_ip`        VARCHAR(64)     DEFAULT NULL,
  `notify_url`       VARCHAR(255)    DEFAULT NULL,
  `return_url`       VARCHAR(255)    DEFAULT NULL,
  `extra_json`       TEXT            DEFAULT NULL,
  `paid_at`          DATETIME(3)     DEFAULT NULL,
  `created_at`       DATETIME(3)     DEFAULT NULL,
  `updated_at`       DATETIME(3)     DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_orders_out_trade_no` (`out_trade_no`),
  KEY `idx_orders_channel`          (`channel`),
  KEY `idx_orders_status`           (`status`),
  KEY `idx_orders_channel_trade_no` (`channel_trade_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `refunds` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `out_trade_no`      VARCHAR(64)     NOT NULL,
  `out_refund_no`     VARCHAR(64)     NOT NULL,
  `channel`           VARCHAR(16)     NOT NULL,
  `refund_amount`     BIGINT          NOT NULL,
  `total_amount`      BIGINT          NOT NULL,
  `reason`            VARCHAR(255)    DEFAULT NULL,
  `status`            VARCHAR(16)     NOT NULL DEFAULT 'INIT',
  `channel_refund_id` VARCHAR(64)     DEFAULT NULL,
  `created_at`        DATETIME(3)     DEFAULT NULL,
  `updated_at`        DATETIME(3)     DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_refunds_out_refund_no`     (`out_refund_no`),
  KEY        `idx_refunds_out_trade_no`      (`out_trade_no`),
  KEY        `idx_refunds_channel_refund_id` (`channel_refund_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `payment_notify_logs` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `channel`      VARCHAR(16)     NOT NULL,
  `event_type`   VARCHAR(64)     NOT NULL,
  `out_trade_no` VARCHAR(64)     DEFAULT NULL,
  `raw_headers`  TEXT            DEFAULT NULL,
  `raw_body`     LONGTEXT        DEFAULT NULL,
  `verified`     TINYINT(1)      NOT NULL DEFAULT 0,
  `created_at`   DATETIME(3)     DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_payment_notify_logs_channel`      (`channel`),
  KEY `idx_payment_notify_logs_event_type`   (`event_type`),
  KEY `idx_payment_notify_logs_out_trade_no` (`out_trade_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
