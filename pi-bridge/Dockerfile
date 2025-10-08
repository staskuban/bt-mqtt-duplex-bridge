# Dockerfile для ELM327 Bridge
FROM golang:1.25-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./

# Скачиваем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o elm327-bridge .

# Финальный образ
FROM alpine:latest

# Устанавливаем необходимые пакеты
RUN apk --no-cache add ca-certificates tzdata

# Создаем пользователя без root прав
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Создаем директорию для конфигурации
RUN mkdir -p /app/config && chown -R appuser:appgroup /app

# Копируем бинарный файл из builder этапа
COPY --from=builder /app/elm327-bridge /app/elm327-bridge

# Копируем пример конфигурации
COPY --from=builder /app/config.example.yaml /app/config/

# Устанавливаем пользователя
USER appuser

# Устанавливаем рабочую директорию
WORKDIR /app

# Команда запуска
CMD ["./elm327-bridge"]