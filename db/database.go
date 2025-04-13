package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"github.com/awhatson15/reminder-bot/models"
)

// DB представляет экземпляр базы данных
type DB struct {
	*sql.DB
}

// NewDB инициализирует соединение с базой данных
func NewDB(dbPath string) (*DB, error) {
	// Создаем директорию для БД, если она не существует
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию для БД: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть базу данных: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	return &DB{db}, nil
}

// InitSchema инициализирует схему базы данных
func (db *DB) InitSchema() error {
	// Создаем таблицу пользователей
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		telegram_id INTEGER UNIQUE NOT NULL,
		username TEXT,
		first_name TEXT,
		last_name TEXT,
		notification_time TEXT DEFAULT '09:00',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("не удалось создать таблицу users: %w", err)
	}

	// Создаем таблицу событий
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		type TEXT NOT NULL,
		event_date TEXT NOT NULL,
		notify_days INTEGER DEFAULT 1,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return fmt.Errorf("не удалось создать таблицу events: %w", err)
	}

	log.Println("Схема базы данных успешно инициализирована")
	return nil
}

// CreateUser создает нового пользователя
func (db *DB) CreateUser(telegramID int64, username, firstName, lastName string) (int64, error) {
	// Проверяем, существует ли пользователь
	var userID int64
	err := db.QueryRow(
		"SELECT id FROM users WHERE telegram_id = ?", 
		telegramID,
	).Scan(&userID)

	if err == nil {
		// Пользователь уже существует
		return userID, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("ошибка при проверке существования пользователя: %w", err)
	}

	// Создаем нового пользователя
	result, err := db.Exec(
		"INSERT INTO users (telegram_id, username, first_name, last_name) VALUES (?, ?, ?, ?)",
		telegramID, username, firstName, lastName,
	)
	if err != nil {
		return 0, fmt.Errorf("ошибка при создании пользователя: %w", err)
	}

	userID, err = result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка при получении ID нового пользователя: %w", err)
	}

	return userID, nil
}

// GetUserByTelegramID получает пользователя по его Telegram ID
func (db *DB) GetUserByTelegramID(telegramID int64) (*models.User, error) {
	user := &models.User{}

	err := db.QueryRow(
		"SELECT id, telegram_id, username, first_name, last_name, notification_time, created_at FROM users WHERE telegram_id = ?",
		telegramID,
	).Scan(&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName, &user.NotificationTime, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка при получении пользователя: %w", err)
	}

	return user, nil
}

// SetUserNotificationTime устанавливает время уведомлений пользователя
func (db *DB) SetUserNotificationTime(userID int64, notificationTime string) error {
	_, err := db.Exec(
		"UPDATE users SET notification_time = ? WHERE id = ?",
		notificationTime, userID,
	)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении времени уведомлений: %w", err)
	}
	return nil
}

// CreateEvent создает новое событие
func (db *DB) CreateEvent(event *models.Event) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO events (user_id, title, type, event_date, notify_days, description) VALUES (?, ?, ?, ?, ?, ?)",
		event.UserID, event.Title, event.Type, event.EventDate, event.NotifyDays, event.Description,
	)
	if err != nil {
		return 0, fmt.Errorf("ошибка при создании события: %w", err)
	}

	eventID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка при получении ID нового события: %w", err)
	}

	return eventID, nil
}

// GetEventsByUserID получает все события пользователя
func (db *DB) GetEventsByUserID(userID int64) ([]*models.Event, error) {
	rows, err := db.Query(
		"SELECT id, user_id, title, type, event_date, notify_days, description, created_at FROM events WHERE user_id = ? ORDER BY event_date",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении событий пользователя: %w", err)
	}
	defer rows.Close()

	events := []*models.Event{}
	for rows.Next() {
		event := &models.Event{}
		err := rows.Scan(
			&event.ID, &event.UserID, &event.Title, &event.Type, 
			&event.EventDate, &event.NotifyDays, &event.Description, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании данных события: %w", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по событиям: %w", err)
	}

	return events, nil
}

// GetEventByID получает событие по его ID
func (db *DB) GetEventByID(eventID int64) (*models.Event, error) {
	event := &models.Event{}

	err := db.QueryRow(
		"SELECT id, user_id, title, type, event_date, notify_days, description, created_at FROM events WHERE id = ?",
		eventID,
	).Scan(&event.ID, &event.UserID, &event.Title, &event.Type, &event.EventDate, &event.NotifyDays, &event.Description, &event.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка при получении события: %w", err)
	}

	return event, nil
}

// UpdateEvent обновляет событие
func (db *DB) UpdateEvent(event *models.Event) error {
	_, err := db.Exec(
		"UPDATE events SET title = ?, type = ?, event_date = ?, notify_days = ?, description = ? WHERE id = ?",
		event.Title, event.Type, event.EventDate, event.NotifyDays, event.Description, event.ID,
	)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении события: %w", err)
	}
	return nil
}

// DeleteEvent удаляет событие
func (db *DB) DeleteEvent(eventID int64) error {
	_, err := db.Exec("DELETE FROM events WHERE id = ?", eventID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении события: %w", err)
	}
	return nil
}

// GetUpcomingEvents получает предстоящие события для отправки уведомлений
func (db *DB) GetUpcomingEvents() ([]*models.Event, error) {
	// Текущая дата
	//currentYear := time.Now().Year()
	currentMonth := time.Now().Month()
	currentDay := time.Now().Day()
	
	// SQL запрос для выборки предстоящих событий
	rows, err := db.Query(`
		SELECT e.id, e.user_id, e.title, e.type, e.event_date, e.notify_days, e.description, 
		       u.telegram_id, u.notification_time
		FROM events e
		JOIN users u ON e.user_id = u.id
		WHERE (
			CAST(strftime('%m', e.event_date) AS INTEGER) = ? AND
			CAST(strftime('%d', e.event_date) AS INTEGER) - e.notify_days = ?
		) OR (
			CAST(strftime('%m', e.event_date) AS INTEGER) = ? AND
			CAST(strftime('%d', e.event_date) AS INTEGER) = ?
		)`,
		currentMonth, currentDay, currentMonth, currentDay)
	
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении предстоящих событий: %w", err)
	}
	defer rows.Close()

	events := []*models.Event{}
	for rows.Next() {
		event := &models.Event{}
		var telegramID int64
		var notificationTime string
		
		err := rows.Scan(
			&event.ID, &event.UserID, &event.Title, &event.Type, 
			&event.EventDate, &event.NotifyDays, &event.Description,
			&telegramID, notificationTime,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании данных предстоящего события: %w", err)
		}
		
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по предстоящим событиям: %w", err)
	}

	return events, nil
}

// GetUsersForNotification получает пользователей для уведомлений в указанное время
func (db *DB) GetUsersForNotification(notificationTime string) ([]*models.User, error) {
	rows, err := db.Query(
		"SELECT id, telegram_id, username, first_name, last_name, notification_time, created_at FROM users WHERE notification_time = ?",
		notificationTime,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении пользователей для уведомлений: %w", err)
	}
	defer rows.Close()

	users := []*models.User{}
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName, 
			&user.LastName, &user.NotificationTime, &user.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании данных пользователя: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по пользователям: %w", err)
	}

	return users, nil
}
