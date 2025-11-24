package pubsub

// Logger defines the logging interface required by the PubSub library.
// Implement this interface to integrate your logging system (zap, logrus, etc.).
//
// Example implementation:
//
//	type ZapLogger struct {
//	    logger *zap.Logger
//	}
//
//	func (l *ZapLogger) Infof(format string, args ...interface{}) {
//	    l.logger.Sugar().Infof(format, args...)
//	}
type Logger interface {
	// Debugf logs debug-level messages with printf-style formatting.
	Debugf(format string, args ...interface{})

	// Infof logs info-level messages with printf-style formatting.
	Infof(format string, args ...interface{})

	// Warnf logs warning-level messages with printf-style formatting.
	Warnf(format string, args ...interface{})

	// Errorf logs error-level messages with printf-style formatting.
	Errorf(format string, args ...interface{})

	// Info logs info-level messages without formatting.
	Info(message string)
}

// NoopLogger is a no-operation logger implementation useful for testing
// or when logging is not desired. All methods are no-ops.
type NoopLogger struct{}

// Debugf implements Logger.Debugf as a no-op.
func (l *NoopLogger) Debugf(_ string, _ ...interface{}) {}

// Infof implements Logger.Infof as a no-op.
func (l *NoopLogger) Infof(_ string, _ ...interface{}) {}

// Warnf implements Logger.Warnf as a no-op.
func (l *NoopLogger) Warnf(_ string, _ ...interface{}) {}

// Errorf implements Logger.Errorf as a no-op.
func (l *NoopLogger) Errorf(_ string, _ ...interface{}) {}

// Info implements Logger.Info as a no-op.
func (l *NoopLogger) Info(_ string) {}
