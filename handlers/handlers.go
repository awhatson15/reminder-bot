package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/awhatson15/reminder-bot/bot"
	"github.com/awhatson15/reminder-bot/models"
	"github.com/awhatson15/reminder-bot/utils"
)

// HandleMessage обрабатывает текстовые сообщения
func (b *bot.Bot) handleMessage(message *tgbotapi.Message) {
	if message.IsCommand() {
		b.handleCommand(message)
		return
	}

	userID := message.From.ID
	chatID := message.Chat.ID
	userState := b.GetUserState(userID)

	switch userState.State {
	case models.StateAddEventTitle:
		// Обработка ввода названия события
		b.SaveUserData(userID, "title", message.Text)
		b.SetUserState(userID, models.StateAddEventType)

		// Показываем варианты типов событий
		keyboard := tgbotapi.NewInlineKeyboardMarkup()
		for _, eventType := range models.EventTypes {
			row := tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(eventType, "type:"+eventType),
			)
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
		}

		msg := tgbotapi.NewMessage(chatID, "Выберите тип события:")
		msg.ReplyMarkup = keyboard
		b.API.Send(msg)

	case models.StateAddEventDate:
		// Обработка ввода даты события
		dateStr := message.Text
		formattedDate, err := utils.FormatDate(dateStr)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ %s. Пожалуйста, введите дату в формате ДД.ММ.ГГГГ:", err))
			b.API.Send(msg)
			return
		}

		b.SaveUserData(userID, "event_date", formattedDate)
		b.SetUserState(userID, models.StateAddEventNotify)

		msg := tgbotapi.NewMessage(chatID, "За сколько дней до события отправить напоминание? (введите число от 1 до 30):")
		b.API.Send(msg)

	case models.StateAddEventNotify:
		// Обработка ввода дней для напоминания
		daysStr := message.Text
		days, err := strconv.Atoi(daysStr)
		if err != nil || days < 1 || days > 30 {
			msg := tgbotapi.NewMessage(chatID, "❌ Пожалуйста, введите число от 1 до 30:")
			b.API.Send(msg)
			return
		}

		b.SaveUserData(userID, "notify_days", days)
		b.SetUserState(userID, models.StateAddEventDesc)

		msg := tgbotapi.NewMessage(chatID, "Введите описание события (или отправьте /skip, чтобы пропустить):")
		b.API.Send(msg)

	case models.StateAddEventDesc:
		// Обработка ввода описания события
		description := message.Text
		if message.Text == "/skip" {
			description = ""
		}

		b.SaveUserData(userID, "description", description)

		// Создаем событие в БД
		userData := userState.CurrentData
		user, err := b.DB.GetUserByTelegramID(userID)
		if err != nil {
			log.Printf("Ошибка при получении пользователя: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при создании события.")
			b.API.Send(msg)
			b.ResetUserState(userID)
			return
		}

		event := &models.Event{
			UserID:      user.ID,
			Title:       userData["title"].(string),
			Type:        userData["type"].(string),
			EventDate:   userData["event_date"].(string),
			NotifyDays:  userData["notify_days"].(int),
			Description: description,
		}

		eventID, err := b.DB.CreateEvent(event)
		if err != nil {
			log.Printf("Ошибка при создании события: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при сохранении события.")
			b.API.Send(msg)
			b.ResetUserState(userID)
			return
		}

		// Форматируем дату для отображения
		displayDate := utils.FormatDisplayDate(event.EventDate)

		successMsg := fmt.Sprintf("✅ Событие успешно добавлено!\n\n"+
			"🔤 Название: %s\n"+
			"🏷 Тип: %s\n"+
			"📅 Дата: %s\n"+
			"🔔 Напоминание: за %d дней\n",
			event.Title, event.Type, displayDate, event.NotifyDays)

		if event.Description != "" {
			successMsg += fmt.Sprintf("📝 Описание: %s\n", event.Description)
		}

		msg := tgbotapi.NewMessage(chatID, successMsg)
		b.API.Send(msg)

		// Возвращаем пользователя в главное меню
		b.ResetUserState(userID)
		b.SendMainMenu(chatID)

	case models.StateSetNotifyTime:
		// Обработка ввода времени для уведомлений
		timeStr := message.Text
		formattedTime, err := utils.ValidateTime(timeStr)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ %s. Пожалуйста, введите время в формате ЧЧ:ММ:", err))
			b.API.Send(msg)
			return
		}

		user, err := b.DB.GetUserByTelegramID(userID)
		if err != nil {
			log.Printf("Ошибка при получении пользователя: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при обновлении настроек.")
			b.API.Send(msg)
			b.ResetUserState(userID)
			return
		}

		err = b.DB.SetUserNotificationTime(user.ID, formattedTime)
		if err != nil {
			log.Printf("Ошибка при обновлении времени уведомлений: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при сохранении настроек.")
			b.API.Send(msg)
			b.ResetUserState(userID)
			return
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Время уведомлений установлено на %s", formattedTime))
		b.API.Send(msg)

		// Возвращаем пользователя в главное меню
		b.ResetUserState(userID)
		b.SendMainMenu(chatID)

	case models.StateEditEventValue:
		// Обработка ввода нового значения для редактирования поля события
		field := userState.CurrentData["field"].(string)
		eventID := userState.CurrentData["event_id"].(int64)

		event, err := b.DB.GetEventByID(eventID)
		if err != nil {
			log.Printf("Ошибка при получении события: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при редактировании события.")
			b.API.Send(msg)
			b.ResetUserState(userID)
			return
		}

		switch field {
		case "title":
			event.Title = message.Text

		case "date":
			formattedDate, err := utils.FormatDate(message.Text)
			if err != nil {
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ %s. Пожалуйста, введите дату в формате ДД.ММ.ГГГГ:", err))
				b.API.Send(msg)
				return
			}
			event.EventDate = formattedDate

		case "notify_days":
			days, err := strconv.Atoi(message.Text)
			if err != nil || days < 1 || days > 30 {
				msg := tgbotapi.NewMessage(chatID, "❌ Пожалуйста, введите число от 1 до 30:")
				b.API.Send(msg)
				return
			}
			event.NotifyDays = days

		case "description":
			event.Description = message.Text
		}

		err = b.DB.UpdateEvent(event)
		if err != nil {
			log.Printf("Ошибка при обновлении события: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при сохранении изменений.")
			b.API.Send(msg)
			b.ResetUserState(userID)
			return
		}

		// Форматируем дату для отображения
		displayDate := utils.FormatDisplayDate(event.EventDate)

		successMsg := fmt.Sprintf("✅ Событие успешно обновлено!\n\n"+
			"🔤 Название: %s\n"+
			"🏷 Тип: %s\n"+
			"📅 Дата: %s\n"+
			"🔔 Напоминание: за %d дней\n",
			event.Title, event.Type, displayDate, event.NotifyDays)

		if event.Description != "" {
			successMsg += fmt.Sprintf("📝 Описание: %s\n", event.Description)
		}

		msg := tgbotapi.NewMessage(chatID, successMsg)
		b.API.Send(msg)

		// Возвращаем пользователя в главное меню
		b.ResetUserState(userID)
		b.SendMainMenu(chatID)

	default:
		// В других состояниях отправляем главное меню
		b.ResetUserState(userID)
		b.SendMainMenu(chatID)
	}
}

// HandleCommand обрабатывает команды бота
func (b *bot.Bot) handleCommand(message *tgbotapi.Message) {
	userID := message.From.ID
	chatID := message.Chat.ID

	switch message.Command() {
	case "start":
		// Регистрируем пользователя если он новый
		_, err := b.DB.CreateUser(
			userID,
			message.From.UserName,
			message.From.FirstName,
			message.From.LastName,
		)
		if err != nil {
			log.Printf("Ошибка при создании пользователя: %v", err)
		}

		// Приветственное сообщение
		welcomeMsg := fmt.Sprintf(
			"👋 Привет, %s!\n\n"+
				"Я бот для напоминания о днях рождения и важных событиях. "+
				"С моей помощью вы не забудете поздравить друзей и близких с праздниками.\n\n"+
				"Используйте меню для управления вашими событиями и напоминаниями:",
			message.From.FirstName,
		)
		msg := tgbotapi.NewMessage(chatID, welcomeMsg)
		b.API.Send(msg)

		// Сбрасываем состояние и отправляем главное меню
		b.ResetUserState(userID)
		b.SendMainMenu(chatID)

	case "help":
		helpMsg := "🤖 *Справка по командам:*\n\n" +
			"/start - запустить бота и показать главное меню\n" +
			"/help - показать справку по командам\n" +
			"/add - добавить новое событие\n" +
			"/list - показать список ваших событий\n" +
			"/settings - настройки уведомлений\n\n" +
			"Вы также можете использовать кнопки меню для более удобной навигации."

		msg := tgbotapi.NewMessage(chatID, helpMsg)
		msg.ParseMode = "Markdown"
		b.API.Send(msg)

	case "add":
		// Начинаем процесс добавления события
		b.SetUserState(userID, models.StateAddEventTitle)
		msg := tgbotapi.NewMessage(chatID, "Введите название события:")
		b.API.Send(msg)

	case "list":
		// Отправляем список событий пользователя
		b.sendEventsList(chatID, userID)

	case "settings":
		// Показываем меню настроек
		b.showSettings(chatID, userID)

	case "skip":
		// Обработка команды пропуска (например, для описания события)
		userState := b.GetUserState(userID)
		if userState.State == models.StateAddEventDesc {
			b.handleMessage(&tgbotapi.Message{
				From:    message.From,
				Chat:    message.Chat,
				Text:    "",
				MessageID: message.MessageID,
			})
		}

	default:
		msg := tgbotapi.NewMessage(chatID, "Неизвестная команда. Используйте /help для списка доступных команд.")
		b.API.Send(msg)
	}
}

// HandleCallbackQuery обрабатывает нажатия на inline-кнопки
func (b *bot.Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	data := callback.Data
	userState := b.GetUserState(userID)

	// Отправляем уведомление о получении запроса
	b.API.Request(tgbotapi.NewCallback(callback.ID, ""))

	switch {
	case data == "add_event":
		// Начинаем процесс добавления события
		b.SetUserState(userID, models.StateAddEventTitle)
		msg := tgbotapi.NewMessage(chatID, "Введите название события:")
		b.API.Send(msg)

	case data == "list_events":
		// Отправляем список событий пользователя
		b.sendEventsList(chatID, userID)

	case data == "settings":
		// Показываем меню настроек
		b.showSettings(chatID, userID)

	case data == "help":
		// Отправляем справку
		helpMsg := "🤖 *Справка по командам:*\n\n" +
			"/start - запустить бота и показать главное меню\n" +
			"/help - показать справку по командам\n" +
			"/add - добавить новое событие\n" +
			"/list - показать список ваших событий\n" +
			"/settings - настройки уведомлений\n\n" +
			"Вы также можете использовать кнопки меню для более удобной навигации."

		msg := tgbotapi.NewMessage(chatID, helpMsg)
		msg.ParseMode = "Markdown"
		b.API.Send(msg)

	case data == "back_to_menu":
		// Возвращаем пользователя в главное меню
		b.ResetUserState(userID)
		b.SendMainMenu(chatID)

	case data == "set_notify_time":
		// Начинаем процесс установки времени уведомлений
		b.SetUserState(userID, models.StateSetNotifyTime)
		msg := tgbotapi.NewMessage(chatID, "Введите время для получения уведомлений в формате ЧЧ:ММ (например, 09:00):")
		b.API.Send(msg)

	case strings.HasPrefix(data, "type:"):
		// Обработка выбора типа события
		if userState.State == models.StateAddEventType {
			eventType := strings.TrimPrefix(data, "type:")
			b.SaveUserData(userID, "type", eventType)
			b.SetUserState(userID, models.StateAddEventDate)

			msg := tgbotapi.NewMessage(chatID, "Введите дату события в формате ДД.ММ.ГГГГ:")
			b.API.Send(msg)
		}

	case strings.HasPrefix(data, "event:"):
		// Обработка выбора события для редактирования или просмотра
		eventIDStr := strings.TrimPrefix(data, "event:")
		eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
		if err != nil {
			log.Printf("Ошибка при парсинге ID события: %v", err)
			return
		}

		event, err := b.DB.GetEventByID(eventID)
		if err != nil || event == nil {
			log.Printf("Ошибка при получении события: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Событие не найдено.")
			b.API.Send(msg)
			return
		}

		// Форматируем дату для отображения
		displayDate := utils.FormatDisplayDate(event.EventDate)

		// Отправляем информацию о событии с кнопками редактирования
		eventMsg := fmt.Sprintf("🗓 *%s*\n\n"+
			"🏷 Тип: %s\n"+
			"📅 Дата: %s\n"+
			"🔔 Напоминание: за %d дней\n",
			event.Title, event.Type, displayDate, event.NotifyDays)

		if event.Description != "" {
			eventMsg += fmt.Sprintf("📝 Описание: %s\n", event.Description)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", fmt.Sprintf("edit:%d", event.ID)),
				tgbotapi.NewInlineKeyboardButtonData("❌ Удалить", fmt.Sprintf("delete:%d", event.ID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад к списку", "list_events"),
				tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "back_to_menu"),
			),
		)

		msg := tgbotapi.NewMessage(chatID, eventMsg)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		b.API.Send(msg)

	case strings.HasPrefix(data, "edit:"):
		// Начало редактирования события
		eventIDStr := strings.TrimPrefix(data, "edit:")
		eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
		if err != nil {
			log.Printf("Ошибка при парсинге ID события: %v", err)
			return
		}

		event, err := b.DB.GetEventByID(eventID)
		if err != nil || event == nil {
			log.Printf("Ошибка при получении события: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Событие не найдено.")
			b.API.Send(msg)
			return
		}

		// Сохраняем ID события в состоянии пользователя
		b.SaveUserData(userID, "event_id", eventID)
		b.SetUserState(userID, models.StateEditEventField)

		// Показываем варианты полей для редактирования
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔤 Название", "edit_field:title"),
				tgbotapi.NewInlineKeyboardButtonData("🏷 Тип", "edit_field:type"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📅 Дата", "edit_field:date"),
				tgbotapi.NewInlineKeyboardButtonData("🔔 Дни напоминания", "edit_field:notify_days"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📝 Описание", "edit_field:description"),
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("event:%d", eventID)),
			),
		)

		msg := tgbotapi.NewMessage(chatID, "Что вы хотите изменить?")
		msg.ReplyMarkup = keyboard
		b.API.Send(msg)

	case strings.HasPrefix(data, "edit_field:"):
		// Обработка выбора поля для редактирования
		field := strings.TrimPrefix(data, "edit_field:")
		b.SaveUserData(userID, "field", field)
		b.SetUserState(userID, models.StateEditEventValue)

		var promptMsg string
		switch field {
		case "title":
			promptMsg = "Введите новое название события:"
		case "type":
			// Для типа показываем кнопки с вариантами
			keyboard := tgbotapi.NewInlineKeyboardMarkup()
			for _, eventType := range models.EventTypes {
				row := tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(eventType, "set_type:"+eventType),
				)
				keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
			}

			msg := tgbotapi.NewMessage(chatID, "Выберите новый тип события:")
			msg.ReplyMarkup = keyboard
			b.API.Send(msg)
			return
		case "date":
			promptMsg = "Введите новую дату события в формате ДД.ММ.ГГГГ:"
		case "notify_days":
			promptMsg = "За сколько дней до события отправлять напоминание? (введите число от 1 до 30):"
		case "description":
			promptMsg = "Введите новое описание события:"
		default:
			promptMsg = "Выберите поле для редактирования:"
		}

		msg := tgbotapi.NewMessage(chatID, promptMsg)
		b.API.Send(msg)

	case strings.HasPrefix(data, "set_type:"):
		// Обработка выбора нового типа события при редактировании
		if userState.State == models.StateEditEventValue && userState.CurrentData["field"] == "type" {
			newType := strings.TrimPrefix(data, "set_type:")
			eventID := userState.CurrentData["event_id"].(int64)

			event, err := b.DB.GetEventByID(eventID)
			if err != nil {
				log.Printf("Ошибка при получении события: %v", err)
				msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при редактировании события.")
				b.API.Send(msg)
				b.ResetUserState(userID)
				return
			}

			event.Type = newType
			err = b.DB.UpdateEvent(event)
			if err != nil {
				log.Printf("Ошибка при обновлении события: %v", err)
				msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при сохранении изменений.")
				b.API.Send(msg)
				b.ResetUserState(userID)
				return
			}

			// Форматируем дату для отображения
			displayDate := utils.FormatDisplayDate(event.EventDate)

			successMsg := fmt.Sprintf("✅ Событие успешно обновлено!\n\n"+
				"🔤 Название: %s\n"+
				"🏷 Тип: %s\n"+
				"📅 Дата: %s\n"+
				"🔔 Напоминание: за %d дней\n",
				event.Title, event.Type, displayDate, event.NotifyDays)

			if event.Description != "" {
				successMsg += fmt.Sprintf("📝 Описание: %s\n", event.Description)
			}

			msg := tgbotapi.NewMessage(chatID, successMsg)
			b.API.Send(msg)

			// Возвращаем пользователя в главное меню
			b.ResetUserState(userID)
			b.SendMainMenu(chatID)
		}

	case strings.HasPrefix(data, "delete:"):
		// Подтверждение удаления события
		eventIDStr := strings.TrimPrefix(data, "delete:")
		eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
		if err != nil {
			log.Printf("Ошибка при парсинге ID события: %v", err)
			return
		}

		// Сохраняем ID события для последующего удаления
		b.SaveUserData(userID, "delete_event_id", eventID)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", fmt.Sprintf("confirm_delete:%d", eventID)),
				tgbotapi.NewInlineKeyboardButtonData("❌ Нет, отменить", fmt.Sprintf("event:%d", eventID)),
			),
		)

		msg := tgbotapi.NewMessage(chatID, "❓ Вы уверены, что хотите удалить это событие? Это действие нельзя отменить.")
		msg.ReplyMarkup = keyboard
		b.API.Send(msg)

	case strings.HasPrefix(data, "confirm_delete:"):
		// Выполнение удаления события
		eventIDStr := strings.TrimPrefix(data, "confirm_delete:")
		eventID, err := strconv.ParseInt(eventIDStr, 10, 64)
		if err != nil {
			log.Printf("Ошибка при парсинге ID события: %v", err)
			return
		}

		err = b.DB.DeleteEvent(eventID)
		if err != nil {
			log.Printf("Ошибка при удалении события: %v", err)
			msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при удалении события.")
			b.API.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(chatID, "✅ Событие успешно удалено!")
		b.API.Send(msg)

		// Обновляем список событий
		b.sendEventsList(chatID, userID)
	}
}

// sendEventsList отправляет список событий пользователя
func (b *bot.Bot) sendEventsList(chatID, userID int64) {
	user, err := b.DB.GetUserByTelegramID(userID)
	if err != nil {
		log.Printf("Ошибка при получении пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при получении списка событий.")
		b.API.Send(msg)
		return
	}

	events, err := b.DB.GetEventsByUserID(user.ID)
	if err != nil {
		log.Printf("Ошибка при получении событий: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при получении списка событий.")
		b.API.Send(msg)
		return
	}

	if len(events) == 0 {
		// Если событий нет
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ Добавить событие", "add_event"),
				tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "back_to_menu"),
			),
		)

		msg := tgbotapi.NewMessage(chatID, "У вас пока нет добавленных событий. Добавьте первое событие!")
		msg.ReplyMarkup = keyboard
		b.API.Send(msg)
		return
	}

	// Создаем клавиатуру с кнопками для каждого события
	keyboard := tgbotapi.NewInlineKeyboardMarkup()
	for _, event := range events {
		// Форматируем дату для отображения
		displayDate := utils.FormatDisplayDate(event.EventDate)
		
		// Пытаемся получить дни до события
		daysLeft, err := utils.DaysUntilEvent(event.EventDate)
		daysInfo := ""
		if err == nil {
			if daysLeft == 0 {
				daysInfo = " (сегодня!)"
			} else {
				daysInfo = fmt.Sprintf(" (через %d дн.)", daysLeft)
			}
		}
		
		buttonText := fmt.Sprintf("%s - %s%s", event.Title, displayDate, daysInfo)
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("event:%d", event.ID)),
		)
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
	}

	// Добавляем кнопки для добавления нового события и возврата в меню
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, 
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить событие", "add_event"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "back_to_menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "🗓 Ваши события:")
	msg.ReplyMarkup = keyboard
	b.API.Send(msg)
}

// showSettings показывает меню настроек
func (b *bot.Bot) showSettings(chatID, userID int64) {
	user, err := b.DB.GetUserByTelegramID(userID)
	if err != nil {
		log.Printf("Ошибка при получении пользователя: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Произошла ошибка при получении настроек.")
		b.API.Send(msg)
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ Изменить время уведомлений", "set_notify_time"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "back_to_menu"),
		),
	)

	settingsMsg := fmt.Sprintf("⚙️ *Настройки:*\n\n"+
		"⏰ Время уведомлений: *%s*\n\n"+
		"Выберите настройку, которую хотите изменить:",
		user.NotificationTime)

	msg := tgbotapi.NewMessage(chatID, settingsMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	b.API.Send(msg)
}
