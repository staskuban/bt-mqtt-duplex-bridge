# Makefile для ELM327 Bridge

# Переменные
APP_NAME=elm327-bridge
DOCKER_IMAGE=elm327-bridge:latest
GO_FILES=$(shell find . -name "*.go" -not -path "./vendor/*")

# Цели по умолчанию
.PHONY: help build test clean docker-build docker-run install deploy

# Справка
help:
	@echo "🚗 ELM327 Bridge - Доступные команды:"
	@echo ""
	@echo "Сборка:"
	@echo "  build      - Сборка приложения для текущей ОС"
	@echo "  build-pi   - Сборка приложения для Raspberry Pi (linux/arm64)"
	@echo "  test       - Запуск тестов"
	@echo "  clean      - Очистка артефактов сборки"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build  - Сборка Docker образа"
	@echo "  docker-run    - Запуск в Docker контейнере"
	@echo "  docker-stop   - Остановка Docker контейнера"
	@echo ""
	@echo "Развертывание:"
	@echo "  install    - Установка на Raspberry Pi"
	@echo "  deploy     - Полное развертывание с Docker"
	@echo ""
	@echo "Разработка:"
	@echo "  fmt        - Форматирование кода"
	@echo "  vet        - Статический анализ кода"
	@echo "  lint       - Проверка стиля кода"

# Сборка приложения
build: $(GO_FILES)
	@echo "🔨 Сборка $(APP_NAME)..."
	@go mod tidy
	@go build -o $(APP_NAME) .
	@echo "✅ Сборка завершена: $(APP_NAME)"

# Сборка для Raspberry Pi (ARM64)
build-pi: $(GO_FILES)
	@echo "🔨 Сборка $(APP_NAME) для Raspberry Pi (linux/arm64)..."
	@go mod tidy
	@env GOOS=linux GOARCH=arm64 go build -o $(APP_NAME) .
	@echo "✅ Сборка для Raspberry Pi завершена: $(APP_NAME)"

# Запуск тестов
test:
	@echo "🧪 Запуск тестов..."
	@go test -v ./...

# Очистка артефактов
clean:
	@echo "🧹 Очистка артефактов..."
	@rm -f $(APP_NAME)
	@go clean
	@echo "✅ Очистка завершена"

# Форматирование кода
fmt:
	@echo "💅 Форматирование кода..."
	@gofmt -w $(GO_FILES)
	@echo "✅ Форматирование завершено"

# Статический анализ
vet:
	@echo "🔍 Статический анализ кода..."
	@go vet ./...
	@echo "✅ Анализ завершен"

# Проверка стиля кода
lint:
	@echo "🔍 Проверка стиля кода..."
	@golangci-lint run
	@echo "✅ Проверка стиля завершена"

# Сборка Docker образа
docker-build:
	@echo "🐳 Сборка Docker образа..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "✅ Docker образ собран: $(DOCKER_IMAGE)"

# Запуск в Docker
docker-run:
	@echo "🚀 Запуск в Docker..."
	@docker-compose up -d
	@echo "✅ Приложение запущено в Docker"

# Остановка Docker контейнера
docker-stop:
	@echo "⏹️ Остановка Docker контейнера..."
	@docker-compose down
	@echo "✅ Контейнер остановлен"

# Полное развертывание с Docker
deploy: docker-build docker-run
	@echo "🚀 Полное развертывание завершено!"
	@echo ""
	@echo "Доступ к сервисам:"
	@echo "  - MQTT брокер: http://localhost:1883"
	@echo "  - Bridge логи: docker-compose logs -f elm327-bridge"
	@echo "  - MQTT логи:   docker-compose logs -f mosquitto"

# Деплой на Raspberry Pi через SCP
deploy-pi: build-pi
	@echo "🚀 Деплой на Raspberry Pi..."
	@echo "📦 Копирование бинарного файла..."
	@scp -i ~/.ssh/id_rsa elm327-bridge user@192.168.28.190:/home/user/elm327/
	@echo "🔧 Копирование конфигурации..."
	@scp -i ~/.ssh/id_rsa config.example.yaml user@192.168.28.190:/home/user/elm327/config.yaml
	@echo "📋 Копирование скриптов..."
	@scp -i ~/.ssh/id_rsa setup.sh user@192.168.28.190:/home/user/elm327/
	@ssh -i ~/.ssh/id_rsa user@192.168.28.190 "chmod +x /home/user/elm327/setup.sh"
	@echo "✅ Деплой завершен!"
	@echo ""
	@echo "На Raspberry Pi выполните:"
	@echo "  cd /home/user/elm327"
	@echo "  ./setup.sh"

# Деплой с пересборкой
deploy-pi-full: clean deploy-pi
	@echo "🔄 Полный деплой с пересборкой завершен!"

# Синхронизация проекта с Raspberry Pi
sync-pi:
	@echo "🔄 Синхронизация проекта с Raspberry Pi..."
	@rsync -avz --exclude='.git' --exclude='*.log' --exclude='elm327-bridge' -e "ssh -i ~/.ssh/id_rsa" . user@192.168.28.190:/home/user/elm327/
	@echo "✅ Синхронизация завершена!"

# Установка и запуск на Raspberry Pi
install-pi: deploy-pi
	@echo "⚙️ Установка и запуск на Raspberry Pi..."
	@ssh -i ~/.ssh/id_rsa user@192.168.28.190 "cd /home/user/elm327 && ./setup.sh"
	@echo "✅ Установка завершена!"

# Установка на Raspberry Pi
install: build
	@echo "📦 Установка на Raspberry Pi..."
	@sudo cp $(APP_NAME) /usr/local/bin/
	@sudo cp config.example.yaml /etc/elm327-bridge.yaml
	@echo "✅ Установка завершена"
	@echo ""
	@echo "Для настройки выполните:"
	@echo "  sudo nano /etc/elm327-bridge.yaml"
	@echo "  sudo systemctl enable elm327-bridge"
	@echo "  sudo systemctl start elm327-bridge"

# Полная проверка проекта
check: fmt vet test
	@echo "✅ Все проверки пройдены"

# Разработка с автоперезагрузкой
dev:
	@echo "🔄 Режим разработки..."
	@air

# Проверка зависимостей
deps:
	@echo "📋 Проверка зависимостей..."
	@go mod verify
	@echo "✅ Зависимости проверены"

# Обновление зависимостей
update-deps:
	@echo "⬆️ Обновление зависимостей..."
	@go get -u ./...
	@go mod tidy
	@echo "✅ Зависимости обновлены"

# Анализ покрытия тестами
coverage:
	@echo "📊 Анализ покрытия тестами..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Отчет покрытия создан: coverage.html"

# Установка инструментов разработки
tools:
	@echo "🔧 Установка инструментов разработки..."
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ Инструменты установлены"

# Показать статус проекта
status:
	@echo "📊 Статус проекта:"
	@echo "  Go версии: $$(go version)"
	@echo "  Модули: $$(go list -m)"
	@echo "  Файлы Go: $$(find . -name "*.go" | wc -l)"
	@echo "  Тесты: $$(find . -name "*_test.go" | wc -l)"
	@echo "  Размер бинарника: $$(du -h $(APP_NAME) 2>/dev/null | cut -f1 || echo "не собран")"
	@echo "  Размер исходного кода: $$(du -sh . 2>/dev/null | cut -f1 || echo "неизвестен")"

# Проверка готовности к развертыванию
ready:
	@echo "🔍 Проверка готовности к развертыванию..."
	@if [ ! -f "$(APP_NAME)" ]; then \
		echo "❌ Бинарный файл не найден. Выполните 'make build'"; \
		exit 1; \
	fi
	@if [ ! -f "config.yaml" ]; then \
		echo "❌ Файл конфигурации не найден. Скопируйте config.example.yaml в config.yaml"; \
		exit 1; \
	fi
	@echo "✅ Готов к развертыванию!"
	@echo "📦 Размер бинарника: $$(du -h $(APP_NAME) | cut -f1)"
	@echo "🔧 Конфигурация: config.yaml"
	@echo "📋 Доступные команды деплоя:"
	@echo "   make deploy-pi      - Деплой на Raspberry Pi"
	@echo "   make deploy-pi-full - Полная пересборка и деплой"
	@echo "   make sync-pi        - Синхронизация проекта"

# Быстрая сборка и запуск
quick: build
	@echo "⚡ Быстрый запуск..."
	@./$(APP_NAME)

# Показать логи Docker контейнера
logs:
	@docker-compose logs -f elm327-bridge

# Мониторинг MQTT сообщений
monitor-mqtt:
	@echo "📡 Мониторинг MQTT сообщений..."
	@docker run -it --rm --network elm327-network eclipse-mosquitto:2.0 mosquitto_sub -h mosquitto -t "car/telemetry/#" -v

# Отправка тестовой команды
test-command:
	@echo "🧪 Отправка тестовой команды..."
	@docker run -it --rm --network elm327-network eclipse-mosquitto:2.0 mosquitto_pub -h mosquitto -t "car/command/test/request" -m '{"command":"010C","correlation_id":"test-123","description":"Test RPM query"}'

.DEFAULT_GOAL := help