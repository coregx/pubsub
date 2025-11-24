-- +goose Up

-- =============================================
-- FreiCON PubSub (Message Bus) Tables
-- =============================================
-- Migration: 00005
-- Description: PubSub service tables (7 tables)
-- Purpose: Internal event-driven message bus
-- =============================================

-- Base tables (no dependencies)

CREATE TABLE IF NOT EXISTS `pubsub_publisher` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(100) NOT NULL,
  `name` varchar(50) NOT NULL,
  `description` varchar(255) NOT NULL,
  `access_key` varchar(50) NOT NULL DEFAULT '',
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Message publishers (microservices)';

CREATE TABLE IF NOT EXISTS `pubsub_subscriber` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `client_id` smallint unsigned NOT NULL COMMENT 'deprecated - for backward compatibility',
  `name` varchar(150) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT 'deprecated',
  `email` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT 'deprecated',
  `phone` varchar(25) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL COMMENT 'deprecated',
  `is_empty_possible` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Message subscribers (consumers)';

-- Topic table (depends on publisher)

CREATE TABLE IF NOT EXISTS `pubsub_topic` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `publisher_id` int unsigned NOT NULL DEFAULT '1',
  `code` varchar(100) NOT NULL,
  `name` varchar(50) NOT NULL,
  `description` varchar(255) NOT NULL,
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`),
  KEY `publisher_id` (`publisher_id`),
  CONSTRAINT `pubsub_topic_ibfk_1`
    FOREIGN KEY (`publisher_id`)
    REFERENCES `pubsub_publisher` (`id`)
    ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Message topics (event types)';

-- Transmitter table (depends on subscriber)

CREATE TABLE IF NOT EXISTS `pubsub_transmitter` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `subscriber_id` int unsigned NOT NULL,
  `callback_url` varchar(255) NOT NULL,
  `tried_at` timestamp NULL DEFAULT NULL,
  `attempts` int unsigned NOT NULL DEFAULT '0',
  `interval` int NOT NULL,
  `is_default` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`),
  KEY `subscriber_id` (`subscriber_id`),
  CONSTRAINT `pubsub_transmitter_ibfk_1`
    FOREIGN KEY (`subscriber_id`)
    REFERENCES `pubsub_subscriber` (`id`)
    ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Message delivery endpoints';

-- Subscription table (depends on subscriber, topic, transmitter)

CREATE TABLE IF NOT EXISTS `pubsub_subscription` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `subscriber_id` int unsigned NOT NULL,
  `topic_id` int unsigned NOT NULL,
  `identifier` varchar(20) NOT NULL,
  `is_active` tinyint(1) NOT NULL,
  `transmitter_id` int unsigned NOT NULL,
  `created_at` timestamp NOT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `topic_id` (`topic_id`),
  KEY `transmitter_id` (`transmitter_id`),
  KEY `subscriber_id` (`subscriber_id`),
  CONSTRAINT `pubsub_subscription_ibfk_3`
    FOREIGN KEY (`transmitter_id`)
    REFERENCES `pubsub_transmitter` (`id`)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `pubsub_subscription_ibfk_4`
    FOREIGN KEY (`subscriber_id`)
    REFERENCES `pubsub_subscriber` (`id`)
    ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `pubsub_subscription_ibfk_5`
    FOREIGN KEY (`topic_id`)
    REFERENCES `pubsub_topic` (`id`)
    ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Subscriber-to-topic mappings';

-- Message table (depends on topic)

CREATE TABLE IF NOT EXISTS `pubsub_message` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `topic_id` int unsigned NOT NULL,
  `identifier` varchar(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `version` varchar(10) NOT NULL DEFAULT '1.0',
  `data` text NOT NULL,
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`),
  KEY `topic_id` (`topic_id`),
  CONSTRAINT `pubsub_message_ibfk_1`
    FOREIGN KEY (`topic_id`)
    REFERENCES `pubsub_topic` (`id`)
    ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Published messages';

-- Queue table (depends on subscription and message)

CREATE TABLE IF NOT EXISTS `pubsub_queue` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `subscription_id` int unsigned NOT NULL,
  `message_id` int unsigned NOT NULL,
  `retry_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `is_complete` int NOT NULL DEFAULT '0',
  `completed_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`),
  KEY `subscription_id` (`subscription_id`),
  KEY `message_id` (`message_id`),
  CONSTRAINT `pubsub_queue_ibfk_1`
    FOREIGN KEY (`subscription_id`)
    REFERENCES `pubsub_subscription` (`id`)
    ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `pubsub_queue_ibfk_2`
    FOREIGN KEY (`message_id`)
    REFERENCES `pubsub_message` (`id`)
    ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Message delivery queue';

-- +goose Down

-- Drop in reverse dependency order
DROP TABLE IF EXISTS `pubsub_queue`;
DROP TABLE IF EXISTS `pubsub_message`;
DROP TABLE IF EXISTS `pubsub_subscription`;
DROP TABLE IF EXISTS `pubsub_transmitter`;
DROP TABLE IF EXISTS `pubsub_topic`;
DROP TABLE IF EXISTS `pubsub_subscriber`;
DROP TABLE IF EXISTS `pubsub_publisher`;
