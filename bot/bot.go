package bot

import (
	"fmt"
	"log"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/yourusername/reminder-bot/db"
	"github.com/yourusername/reminder-bot/models"
)

// Bot представляет Telegram бота
type Bot struct {
	API         *tgbotapi.BotAPI
	DB          *db.DB
	UserStates  map[int64]*models.UserState
	statesMutex sync.RWMutex
}

// NewBot создает нового бота
func NewBot(token string, database *db.DB) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании бота: %w", err)
	}

	return &Bot{
		API:        api,
		DB:         database,
		UserStates: make(map[int64]*models.UserState),
	}, nil
}

// Start запускает бота
func (b *Bot) Start() {
	log.Printf("Авторизован как %s", b.API.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.API.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			go b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			go b.handleCallbackQuery(update.CallbackQuery)
		}
	}
}

// GetUserState получает состояние пользователя
func (b *Bot) GetUserState(userID int64) *models.UserState {
	b.statesMutex.RLock()
	state, exists := b.UserStates[userID]
	b.statesMutex.RUnlock()

	if !exists {
		state = &models.UserState{
			State:       models.StateDefault,
			CurrentData: make(map[string]interface{}),
		}
		b.statesMutex.Lock()
		b.UserStates[userID] = state
		b.statesMutex.Unlock()
	}

	return state
}

// SetUserState устанавливает состояние пользователя
func (b *Bot) SetUserState(userID int64, state string) {
	b.statesMutex.Lock()
	defer b.statesMutex.Unlock()

	_, exists := b.UserStates[userID]
	if !exists {
		b.UserStates[userID] = &models.UserState{
			State:       state,
			CurrentData: make(map[string]interface{}),
		}
	} else {
		b.UserStates[userID].State = state
	}
}

// ResetUserState сбрасывает состояние пользователя
func (b *Bot) ResetUserState(userID int64) {
	b.statesMutex.Lock()
	defer b.statesMutex.Unlock()

	b.UserStates[userID] = &models.UserState{
		State:       models.StateDefault,
		CurrentData: make(map[string]interface{}),
	}
}

// SaveUserData сохраняет данные в состоянии пользователя
func (b *Bot) SaveUserData(userID int64, key string, value interface{}) {
	b.statesMutex.Lock()
	defer b.statesMutex.Unlock()

	state, exists := b.UserStates[userID]
	if !exists {
		state = &models.UserState{
			State:       models.StateDefault,
			CurrentData: make(map[string]interface{}),
		}
		b.UserStates[userID] = state
	}

	state.CurrentData[key] = value
}

// SendMainMenu отправляет основное меню
func (b *Bot) SendMainMenu(chatID int64) error {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Добавить событие", "add_event"),
			tgbotapi.NewInlineKeyboardButtonData("Мои события", "list_events"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Настройки", "settings"),
			tgbotapi.NewInlineKeyboardButtonData("Помощь", "help"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Что вы хотите сделать?")
	msg.ReplyMarkup = keyboard

	_, err := b.API.Send(msg)
	if err != nil {
		return fmt.Errorf("ошибка при отправке меню: %w", err)
	}

	return nil
}

// SendNotification отправляет уведомление о предстоящем событии
func (b *Bot) SendNotification(user *models.User, event *models.Event, daysLeft int) error {
	var messageText string
	
	if daysLeft == 0 {
		// Событие сегодня
		messageText = fmt.Sprintf("🎉 Сегодня: %s (%s)\n%s", 
			event.Title, event.Type, event.Description)
	} else {
		// Уведомление за N дней
		messageText = fmt.Sprintf("🔔 Через %d дней: %s (%s)\n%s", 
			daysLeft, event.Title, event.Type, event.Description)
	}
	
	msg := tgbotapi.NewMessage(user.TelegramID, messageText)
	_, err := b.API.Send(msg)
	
	if err != nil {
		return fmt.Errorf("ошибка при отправке уведомления: %w", err)
	}
	
	return nil
}

// CheckAndSendNotifications проверяет и отправляет уведомления
func (b *Bot) CheckAndSendNotifications() error {
	// Получаем предстоящие события
	events, err := b.DB.GetUpcomingEvents()
	if err != nil {
		return fmt.Errorf("ошибка при получении предстоящих событий: %w", err)
	}
	
	for _, event := range events {
		// Получаем пользователя
		user, err := b.DB.GetUserByTelegramID(event.UserID)
		if err != nil {
			log.Printf("Ошибка при получении пользователя %d: %v", event.UserID, err)
			continue
		}
		
		if user == nil {
			continue
		}
		
		// Определяем, сколько дней осталось до события
		daysLeft, err := utils.DaysUntilEvent(event.EventDate)
		if err != nil {
			log.Printf("Ошибка при расчете дней до события %d: %v", event.ID, err)
			continue
		}
		
		// Отправляем уведомление, если осталось столько дней, сколько указано в настройках
		// или если событие сегодня
		if daysLeft == event.NotifyDays || daysLeft == 0 {
			err = b.SendNotification(user, event, daysLeft)
			if err != nil {
				log.Printf("Ошибка при отправке уведомления для события %d: %v", event.ID, err)
			}
		}
	}
	
	return nil
}