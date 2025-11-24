-- +goose Up
-- Service: PubSub  
-- Migration: Complete PubSub v2.0 (parts missing from 00018)
-- Date: 2025-10-02


ALTER TABLE pubsub_queue
ADD COLUMN status ENUM('pending', 'sent', 'failed') NOT NULL DEFAULT 'pending' AFTER message_id,
ADD COLUMN attempt_count INT NOT NULL DEFAULT 0 AFTER status,
ADD COLUMN last_attempt_at TIMESTAMP NULL AFTER attempt_count,
ADD COLUMN next_retry_at TIMESTAMP NULL AFTER last_attempt_at,
ADD COLUMN last_error TEXT NULL AFTER next_retry_at,
ADD COLUMN expires_at TIMESTAMP NOT NULL DEFAULT (CURRENT_TIMESTAMP + INTERVAL 24 HOUR) AFTER last_error,
ADD COLUMN sequence_number BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER expires_at,
ADD COLUMN operation_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP AFTER sequence_number,
ADD INDEX idx_status_attempt (status, attempt_count, last_attempt_at),
ADD INDEX idx_next_retry (status, next_retry_at),
ADD INDEX idx_expires (expires_at, status),
ADD INDEX idx_sequence_ordering (subscription_id, sequence_number),
ADD INDEX idx_guaranteed_delivery (subscription_id, status, operation_timestamp);

ALTER TABLE pubsub_topic
ADD COLUMN message_count BIGINT NOT NULL DEFAULT 0 AFTER description,
ADD COLUMN last_publish_at TIMESTAMP NULL AFTER message_count;

CREATE TABLE IF NOT EXISTS pubsub_notification_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  subscription_id INT UNSIGNED NOT NULL,
  message_id INT UNSIGNED NOT NULL,
  identifier VARCHAR(100) NOT NULL,
  topic_code VARCHAR(50) NOT NULL,
  subscriber_type ENUM('client', 'service') NOT NULL DEFAULT 'client',
  subscriber_id INT UNSIGNED NOT NULL,
  delivery_method ENUM('webhook', 'grpc', 'pull') NOT NULL DEFAULT 'webhook',
  status ENUM('pending', 'sent', 'failed', 'skipped') NOT NULL DEFAULT 'pending',
  skipped_reason VARCHAR(255) NULL,
  sent_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  INDEX idx_subscription (subscription_id, created_at),
  INDEX idx_identifier (identifier, topic_code, created_at),
  INDEX idx_subscriber (subscriber_id, subscriber_type, created_at),
  INDEX idx_status (status, created_at),
  CONSTRAINT fk_notification_subscription FOREIGN KEY (subscription_id) REFERENCES pubsub_subscription (id) ON DELETE CASCADE,
  CONSTRAINT fk_notification_message FOREIGN KEY (message_id) REFERENCES pubsub_message (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS pubsub_notification_log;
ALTER TABLE pubsub_topic DROP COLUMN IF EXISTS last_publish_at, DROP COLUMN IF EXISTS message_count;
ALTER TABLE pubsub_queue DROP INDEX IF EXISTS idx_guaranteed_delivery, DROP INDEX IF EXISTS idx_sequence_ordering, DROP INDEX IF EXISTS idx_expires, DROP INDEX IF EXISTS idx_next_retry, DROP INDEX IF EXISTS idx_status_attempt, DROP COLUMN IF EXISTS operation_timestamp, DROP COLUMN IF EXISTS sequence_number, DROP COLUMN IF EXISTS expires_at, DROP COLUMN IF EXISTS last_error, DROP COLUMN IF EXISTS next_retry_at, DROP COLUMN IF EXISTS last_attempt_at, DROP COLUMN IF EXISTS attempt_count, DROP COLUMN IF EXISTS status;
ALTER TABLE pubsub_message DROP INDEX IF EXISTS idx_identifier_topic;
