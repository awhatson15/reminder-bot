FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o remindersbot .

FROM alpine:latest

# Устанавливаем необходимые пакеты для SQLite
RUN apk --no-cache add ca-certificates tzdata sqlite

WORKDIR /app

# Копируем исполняемый файл из стадии сборки
COPY --from=builder /app/remindersbot .

# Создаем директорию для данных
RUN mkdir -p /app/data

# Устанавливаем переменные окружения
ENV BOT_TOKEN=""
ENV DATABASE_PATH="/app/data/reminder.db"
ENV DEFAULT_NOTIFY_TIME="09:00"

# Запускаем приложение
CMD ["./remindersbot"]
