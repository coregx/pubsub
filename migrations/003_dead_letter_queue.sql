-- +goose Up
-- Service: pubsub
-- Description: Create Dead Letter Queue (DLQ) table for failed messages
-- Related: migration 00019 (retry logic)
-- Purpose: Store messages that exceeded retry threshold for manual investigation

-- Create DLQ table
CREATE TABLE IF NOT EXISTS pubsub_dlq (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,

    -- References
    subscription_id BIGINT NOT NULL,
    message_id BIGINT NOT NULL,
    original_queue_id BIGINT NOT NULL COMMENT 'Reference to original pubsub_queue.id',

    -- Failure information
    attempt_count INT NOT NULL DEFAULT 0 COMMENT 'Total attempts before moving to DLQ',
    last_error TEXT COMMENT 'Last error message',
    failure_reason VARCHAR(500) NOT NULL COMMENT 'Why moved to DLQ (max attempts, expiration, etc)',

    -- Timing
    first_attempt_at TIMESTAMP NOT NULL COMMENT 'When first delivery was attempted',
    last_attempt_at TIMESTAMP NOT NULL COMMENT 'When last attempt failed',
    moved_to_dlq_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'When moved to DLQ',

    -- Denormalized data for easy access
    message_data TEXT NOT NULL COMMENT 'Original message payload (JSON)',
    callback_url VARCHAR(500) NOT NULL COMMENT 'Target webhook URL',

    -- Resolution tracking
    is_resolved BOOLEAN NOT NULL DEFAULT FALSE COMMENT 'Manual resolution flag',
    resolved_at TIMESTAMP NULL COMMENT 'When manually resolved',
    resolved_by VARCHAR(255) DEFAULT NULL COMMENT 'Who resolved (user/system)',
    resolution_note TEXT DEFAULT NULL COMMENT 'Resolution explanation',

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Indexes for common queries
    INDEX idx_subscription_id (subscription_id),
    INDEX idx_message_id (message_id),
    INDEX idx_moved_to_dlq_at (moved_to_dlq_at),
    INDEX idx_is_resolved (is_resolved),
    INDEX idx_failure_reason (failure_reason(255)),

    -- Composite indexes for filtering
    INDEX idx_subscription_unresolved (subscription_id, is_resolved, moved_to_dlq_at),
    INDEX idx_old_unresolved (is_resolved, moved_to_dlq_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
COMMENT='Dead Letter Queue for failed PubSub messages (migration 00021)';

-- +goose Down
DROP TABLE IF EXISTS pubsub_dlq;
