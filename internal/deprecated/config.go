package pubsub

// Config contains configuration for PubSub system
// This allows the library to be used in different projects
type Config struct {
	// Application name for notifications and messages
	ApplicationName string

	// Notification settings
	NotificationSubject   string
	NotificationSmsPrefix string

	// Logger configuration
	LogLevel string
}

// DefaultConfig returns default configuration for FreiCON project
func DefaultConfig() Config {
	return Config{
		ApplicationName:       "FreiCON",
		NotificationSubject:   "FreiCON недоступность вашего шлюза для уведомлений",
		NotificationSmsPrefix: "FreiCON срочное административное сообщение",
		LogLevel:              "info",
	}
}

// NewConfig creates a new configuration with custom application name
func NewConfig(appName string) Config {
	return Config{
		ApplicationName:       appName,
		NotificationSubject:   appName + " gateway unavailability notification",
		NotificationSmsPrefix: appName + " urgent administrative message",
		LogLevel:              "info",
	}
}
