.PHONY: refresh full-refresh build up down logs build-front commit migrate db app-logs

# --- быстрый диплой ---
refresh:
	git pull origin master
	docker compose build app
	docker compose stop app
	docker compose up -d --no-deps app
	docker compose logs -f app

# --- полный рефреш (без удаления volumes!) ---
full-refresh:
	git pull origin master
	docker compose down
	docker compose build --no-cache
	docker compose up -d
	$(MAKE) migrate
	docker compose logs -f app

# --- сборка всех контейнеров ---
build:
	docker compose build

# --- поднять сервисы ---
up:
	docker compose up -d

# --- остановить ---
down:
	docker compose down

# --- логи всех сервисов ---
logs:
	docker compose logs -f

# --- логи только приложения ---
app-logs:
	docker logs --tail=100 -f chatra_ai_bridge_app

# --- Git ---
commit:
	git add .
	git commit -m "$${m:-update}"
	git push origin master

# --- миграции ---
migrate:
	cat migrations/*.sql | docker exec -i chatra_ai_bridge_db psql -U $$POSTGRES_USER -d $$POSTGRES_DB

# --- зайти в PostgreSQL ---
db:
	docker exec -it chatra_ai_bridge_db psql -U $$POSTGRES_USER -d $$POSTGRES_DB