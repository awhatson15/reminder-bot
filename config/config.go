package config

import (
	"os"
	"strconv"
)

// Config содержит настройки приложения
type Config struct {
	BotToken         string
	DatabasePath     string
	LogLevel         string
	DefaultNotifyTime string
}

// LoadConfig загружает конфигурацию из переменных окружения
func LoadConfig() *Config {
	botToken := getEnv("BOT_TOKEN", "")
	databasePath := getEnv("DATABASE_PATH", "./data/reminder.db")
	logLevel := getEnv("LOG_LEVEL", "info")
	defaultNotifyTime := getEnv("DEFAULT_NOTIFY_TIME", "09:00")

	return &Config{
		BotToken:         botToken,
		DatabasePath:     databasePath,
		LogLevel:         logLevel,
		DefaultNotifyTime: defaultNotifyTime,
	}
}

// Получение переменной окружения с возможностью установки значения по умолчанию
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetEnvInt получение числового значения из переменной окружения
func GetEnvInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	
	return value
}
