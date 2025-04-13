package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FormatDate форматирует дату события
func FormatDate(dateStr string) (string, error) {
	// Ожидаем дату в формате ДД.ММ.ГГГГ
	parts := strings.Split(dateStr, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("неверный формат даты, используйте ДД.ММ.ГГГГ")
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil || day < 1 || day > 31 {
		return "", fmt.Errorf("неверный день")
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return "", fmt.Errorf("неверный месяц")
	}

	year, err := strconv.Atoi(parts[2])
	if err != nil || year < 1900 || year > 2100 {
		return "", fmt.Errorf("неверный год")
	}

	// Проверка на валидность даты
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Day() != day || date.Month() != time.Month(month) || date.Year() != year {
		return "", fmt.Errorf("несуществующая дата")
	}

	// Возвращаем дату в формате YYYY-MM-DD для хранения в БД
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day), nil
}

// FormatDisplayDate форматирует дату для отображения пользователю
func FormatDisplayDate(dbDate string) string {
	if dbDate == "" {
		return ""
	}

	// Преобразуем из YYYY-MM-DD в ДД.ММ.YYYY
	date, err := time.Parse("2006-01-02", dbDate)
	if err != nil {
		return dbDate // Возвращаем как есть, если не смогли распарсить
	}

	return date.Format("02.01.2006")
}

// ValidateTime проверяет формат времени
func ValidateTime(timeStr string) (string, error) {
	// Ожидаем время в формате ЧЧ:ММ
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("неверный формат времени, используйте ЧЧ:ММ")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return "", fmt.Errorf("неверный час (0-23)")
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return "", fmt.Errorf("неверная минута (0-59)")
	}

	// Форматируем время в стандартный формат ЧЧ:ММ
	return fmt.Sprintf("%02d:%02d", hour, minute), nil
}

// DaysUntilEvent возвращает количество дней до события
func DaysUntilEvent(eventDate string) (int, error) {
	// Преобразуем из YYYY-MM-DD
	date, err := time.Parse("2006-01-02", eventDate)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	
	// Устанавливаем тот же год для ежегодных событий
	eventThisYear := time.Date(now.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	
	// Если дата уже прошла в этом году, берем дату на следующий год
	if eventThisYear.Before(now) {
		eventThisYear = time.Date(now.Year()+1, date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	}
	
	// Вычисляем разницу в днях
	days := int(eventThisYear.Sub(now).Hours() / 24)
	
	return days, nil
}

// GetCurrentTimeForCron возвращает текущее время в формате для cron-задач
func GetCurrentTimeForCron() string {
	now := time.Now()
	return fmt.Sprintf("%d %d * * *", now.Minute(), now.Hour())
}

// Добавляем методы handleMessage и handleCallbackQuery в структуру Bot

// handleMessage обрабатывает входящие сообщения
func (b *Bot) handleMessage(message *tgbotapi.Message) error {
    return handlers.HandleMessage(b.Client, b.DB, message)
}

// handleCallbackQuery обрабатывает входящие callback запросы
func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) error {
    return handlers.HandleCallbackQuery(b.Client, b.DB, query)
}