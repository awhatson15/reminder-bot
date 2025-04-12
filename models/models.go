package models

import (
	"time"
)

// User представляет информацию о пользователе
type User struct {
	ID              int64
	TelegramID      int64
	Username        string
	FirstName       string
	LastName        string
	NotificationTime string
	CreatedAt       time.Time
}

// Event представляет информацию о событии
type Event struct {
	ID          int64
	UserID      int64
	Title       string
	Type        string
	EventDate   string
	NotifyDays  int
	Description string
	CreatedAt   time.Time
}

// EventTypes возможные типы событий
var EventTypes = []string{
	"День рождения",
	"Встреча",
	"Праздник",
	"Годовщина",
	"Другое",
}

// UserState хранит состояние диалога с пользователем
type UserState struct {
	State       string
	CurrentData map[string]interface{}
}

// Возможные состояния диалога
const (
	StateDefault         = "default"
	StateAddEvent        = "add_event"
	StateAddEventTitle   = "add_event_title"
	StateAddEventType    = "add_event_type"
	StateAddEventDate    = "add_event_date"
	StateAddEventNotify  = "add_event_notify"
	StateAddEventDesc    = "add_event_desc"
	StateEditEvent       = "edit_event"
	StateEditEventField  = "edit_event_field"
	StateEditEventValue  = "edit_event_value"
	StateSettings        = "settings"
	StateSetNotifyTime   = "set_notify_time"
)
