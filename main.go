package main

import (
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/awhatson15/reminder-bot/bot"
	"github.com/awhatson15/reminder-bot/config"
	"github.com/awhatson15/reminder-bot/db"
)

func main() {
	// Загружаем конфигурацию
	cfg := config.LoadConfig()

	// Настраиваем логирование
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Запуск ReminderBot...")

	// Инициализируем базу данных
	database, err := db.NewDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Ошибка при инициализации базы данных: %v", err)
	}
	defer database.Close()

	// Создаем схему базы данных
	err = database.InitSchema()
	if err != nil {
		log.Fatalf("Ошибка при создании схемы базы данных: %v", err)
	}

	// Создаем экземпляр бота
	telegramBot, err := bot.NewBot(cfg.BotToken, database)
	if err != nil {
		log.Fatalf("Ошибка при создании бота: %v", err)
	}

	// Настраиваем планировщик для ежедневной проверки и отправки уведомлений
	scheduler := cron.New()
	
	// Запускаем проверку каждую минуту для уведомлений
	_, err = scheduler.AddFunc("* * * * *", func() {
		currentTime := time.Now().Format("15:04")
		log.Printf("Проверка уведомлений для времени %s", currentTime)
		
		// Получаем всех пользователей с установленным временем уведомлений
		users, err := database.GetUsersForNotification(currentTime)
		if err != nil {
			log.Printf("Ошибка при получении пользователей для уведомлений: %v", err)
			return
		}
		
		for _, user := range users {
			// Проверяем события для каждого пользователя и отправляем уведомления
			events, err := database.GetEventsByUserID(user.ID)
			if err != nil {
				log.Printf("Ошибка при получении событий пользователя %d: %v", user.ID, err)
				continue
			}
			
			for _, event := range events {
				// Проверяем, нужно ли отправлять уведомление сегодня
				daysLeft, err := utils.DaysUntilEvent(event.EventDate)
				if err != nil {
					log.Printf("Ошибка при расчете дней до события %d: %v", event.ID, err)
					continue
				}
				
				// Отправляем уведомление, если осталось столько дней, сколько указано в настройках
				// или если событие сегодня
				if daysLeft == event.NotifyDays || daysLeft == 0 {
					err = telegramBot.SendNotification(user, event, daysLeft)
					if err != nil {
						log.Printf("Ошибка при отправке уведомления для события %d: %v", event.ID, err)
					}
				}
			}
		}
	})
	
	if err != nil {
		log.Printf("Ошибка при настройке планировщика: %v", err)
	}
	
	// Запускаем планировщик
	scheduler.Start()
	defer scheduler.Stop()

	// Запускаем бота
	log.Println("Бот успешно запущен")
	telegramBot.Start()
}
