package logger

import "github.com/sirupsen/logrus"

// Config is the logger configuration (simplified version)
type Config struct {
	Level string `json:"level"` // Log level: debug, info, warn, error (default: info)
}

// TelegramConfig controls optional Telegram log notifications.
type TelegramConfig struct {
	Enabled  bool     `json:"enabled"`
	BotToken string   `json:"bot_token"`
	ChatID   int64    `json:"chat_id"`
	Levels   []string `json:"levels"`
}

// SetDefaults sets default values
func (c *Config) SetDefaults() {
	if c.Level == "" {
		c.Level = "info"
	}
}

// GetLogrusLevels converts configured level names into logrus levels.
func (c *TelegramConfig) GetLogrusLevels() []logrus.Level {
	if c == nil || len(c.Levels) == 0 {
		return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
	}

	levels := make([]logrus.Level, 0, len(c.Levels))
	for _, levelName := range c.Levels {
		level, err := logrus.ParseLevel(levelName)
		if err == nil {
			levels = append(levels, level)
		}
	}
	if len(levels) == 0 {
		return []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
	}
	return levels
}
