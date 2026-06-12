# Todoshka — дизайн-спецификация

**Дата:** 2026-06-12
**Проект:** `~/IdeaProjects/todoshka/`
**Тип:** веб-приложение (задачи + заметки с общим доступом)

## Цель

Веб-приложение, объединяющее канбан-доску задач с текстовыми заметками в стиле Markdown. Поддерживает многопользовательский режим с полным соавторством на расшаренных ресурсах.

## Стек

| Слой | Технология |
|---|---|
| Frontend | Vanilla HTML/CSS/JS (без сборки, без зависимостей) |
| Markdown | `marked` (vendor-копия в `web/vendor/`) |
| Backend | Go 1.21+, `net/http` + `database/sql` |
| Драйвер БД | `github.com/mattn/go-sqlite3` |
| Аутентификация | `github.com/golang-jwt/jwt/v5` + `golang.org/x/crypto/bcrypt` |
| База данных | SQLite (один файл `data/todoshka.db`) |

## Структура проекта

```
todoshka/
├── go.mod
├── main.go
├── internal/
│   ├── db/
│   │   ├── db.go
│   │   ├── users.go
│   │   ├── tasks.go
│   │   ├── notes.go
│   │   └── sharing.go
│   ├── auth/
│   │   └── auth.go
│   ├── handlers/
│   │   ├── auth.go
│   │   ├── tasks.go
│   │   ├── notes.go
│   │   └── share.go
│   └── models/
│       └── models.go
├── web/
│   ├── index.html
│   ├── app.js
│   ├── style.css
│   └── vendor/
│       └── marked.min.js
└── data/
    └── todoshka.db        (в .gitignore)
```

## Модель данных (SQLite)

```sql
CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT    UNIQUE NOT NULL,
  password_hash TEXT    NOT NULL,
  created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tasks (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  owner_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       TEXT    NOT NULL,
  description TEXT,
  status      TEXT    NOT NULL DEFAULT 'todo',  -- 'todo' | 'in_progress' | 'done'
  priority    TEXT    NOT NULL DEFAULT 'medium', -- 'low' | 'medium' | 'high'
  due_date    DATE,
  position    INTEGER NOT NULL DEFAULT 0,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE subtasks (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id  INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  title    TEXT    NOT NULL,
  done     BOOLEAN NOT NULL DEFAULT 0,
  position INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE task_tags (
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  tag     TEXT    NOT NULL,
  PRIMARY KEY (task_id, tag)
);

CREATE TABLE notes (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  owner_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title      TEXT    NOT NULL,
  body_md    TEXT    NOT NULL,
  pinned     BOOLEAN NOT NULL DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE note_versions (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id   INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  title     TEXT    NOT NULL,
  body_md   TEXT    NOT NULL,
  editor_id INTEGER NOT NULL REFERENCES users(id),
  saved_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE note_task_links (
  note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  PRIMARY KEY (note_id, task_id)
);

CREATE TABLE shares (
  resource_type TEXT    NOT NULL,  -- 'task' | 'note'
  resource_id   INTEGER NOT NULL,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (resource_type, resource_id, user_id)
);
```

**Правила доступа:**
- Владелец (`owner_id`) может читать, редактировать, удалять, шарить.
- Наличие записи в `shares` для пользователя = полный доступ на чтение/редактирование.
- Удалить может только владелец.

## REST API

Все защищённые эндпоинты требуют заголовок `Authorization: Bearer <JWT>`. Формат ответа — JSON. Единый формат ошибки: `{ "error": "message", "code": "VALIDATION_FAILED" }`.

### Аутентификация

| Метод | Путь | Тело | Ответ |
|---|---|---|---|
| POST | `/api/register` | `{username, password}` | 201 `{user, token}` |
| POST | `/api/login` | `{username, password}` | 200 `{user, token}` |
| GET | `/api/me` | — | 200 `{user}` |

JWT живёт 30 дней. Хранится в `localStorage` на клиенте.

### Пользователи

| Метод | Путь | Ответ |
|---|---|---|
| GET | `/api/users?q=...` | 200 `[{id, username}]` (autocomplete для шаринга) |

### Задачи

| Метод | Путь | Тело | Ответ |
|---|---|---|---|
| GET | `/api/tasks?status=&tag=` | — | 200 `[Task]` (только свои + расшаренные) |
| POST | `/api/tasks` | `{title, description?, status?, priority?, due_date?, tags?}` | 201 `Task` |
| GET | `/api/tasks/:id` | — | 200 `Task` (с subtasks, tags, linked_notes) |
| PATCH | `/api/tasks/:id` | `{...partial}` | 200 `Task` |
| DELETE | `/api/tasks/:id` | — | 204 |
| POST | `/api/tasks/:id/subtasks` | `{title}` | 201 `Subtask` |
| PATCH | `/api/tasks/:id/subtasks/:sid` | `{done?, title?}` | 200 `Subtask` |
| DELETE | `/api/tasks/:id/subtasks/:sid` | — | 204 |
| POST | `/api/tasks/:id/tags` | `{tag}` | 204 |
| DELETE | `/api/tasks/:id/tags/:tag` | — | 204 |
| POST | `/api/tasks/:id/notes/:nid` | — | 204 (прикрепить заметку) |
| DELETE | `/api/tasks/:id/notes/:nid` | — | 204 |

### Заметки

| Метод | Путь | Тело | Ответ |
|---|---|---|---|
| GET | `/api/notes?q=&tag=&pinned=` | — | 200 `[Note]` |
| POST | `/api/notes` | `{title, body_md}` | 201 `Note` |
| GET | `/api/notes/:id` | — | 200 `Note` (с linked_tasks) |
| PATCH | `/api/notes/:id` | `{title?, body_md?, pinned?}` | 200 `Note` (сохраняет версию) |
| DELETE | `/api/notes/:id` | — | 204 |
| GET | `/api/notes/:id/versions` | — | 200 `[NoteVersion]` |
| POST | `/api/notes/:id/restore/:vid` | — | 200 `Note` |
| POST | `/api/notes/:id/tags` | `{tag}` | 204 |
| DELETE | `/api/notes/:id/tags/:tag` | — | 204 |
| POST | `/api/notes/:id/tasks/:tid` | — | 204 |
| DELETE | `/api/notes/:id/tasks/:tid` | — | 204 |

### Шаринг

| Метод | Путь | Тело | Ответ |
|---|---|---|---|
| POST | `/api/share` | `{resource_type, resource_id, username}` | 204 |
| DELETE | `/api/share` | `{resource_type, resource_id, user_id}` | 204 |
| GET | `/api/shared` | — | 200 `[mixed]` (ресурсы, расшаренные со мной) |

Шаринг = полное соавторство. Только владелец может шарить и удалять.

## Фронтенд

Одна HTML-страница, динамический рендеринг через `innerHTML`, hash-роутинг. Никаких фреймворков.

**Модули `app.js`:**
- `router.js` — слушает `hashchange`, выбирает view
- `api.js` — `fetch`-обёртка с подстановкой JWT
- `store.js` — текущий пользователь, активный раздел
- `views/login.js`, `tasks.js`, `task.js`, `notes.js`, `note.js`, `shared.js`, `search.js`
- `components/sidebar.js`, `taskCard.js`, `noteCard.js`, `modal.js`, `toast.js`

**Роуты:**
- `#/login`, `#/register`
- `#/tasks` — kanban-доска
- `#/tasks/:id` — детальный вид задачи
- `#/notes` — список заметок
- `#/notes/:id` — редактор заметки
- `#/shared` — расшаренное со мной
- `#/search?q=...`

**Боковая панель (sidebar):** Задачи / Заметки / Общие / Поиск. Сверху — текущий пользователь и кнопка «Выйти».

**Kanban:** три колонки (todo / in_progress / done), drag-and-drop через нативный HTML5 API. На drop → `PATCH /api/tasks/:id` с новым `status` и `position`.

**Markdown-редактор:** `textarea` слева + рендеринг через `marked` справа. Переключение «редактировать/предпросмотр» кнопкой.

**Стили:** CSS Grid + Flexbox, без препроцессоров. В `style.css` — переменные для цветов и отступов.

## Ключевые сценарии

### Регистрация/логин
1. Форма логина. Кнопка «Создать аккаунт» → форма регистрации.
2. Бэк: bcrypt-хеш, выдача JWT.
3. Токен в `localStorage`, автоподстановка в `api.js`.
4. При 401 — очистка токена, редирект на `#/login`.

### Шаринг
1. На детальном виде кнопка «Поделиться» → модалка с поиском.
2. `GET /api/users?q=...` (debounce 200мс) → autocomplete.
3. Выбор пользователя → `POST /api/share`.
4. На странице появляется плашка «Доступ: user1, user2» + кнопка «Отозвать» (только владелец).

### История версий заметки
1. На детальном виде кнопка «История» → список (автор, время, превью).
2. Клик по версии → превью + кнопка «Восстановить».
3. `POST /api/notes/:id/restore/:vid` → обновляет `notes` + пишет новую версию (старая не удаляется).

### Связь заметка↔задача
1. В детальном виде заметки — блок «Прикреплённые задачи»: список + поле добавления.
2. В детальном виде задачи — аналогичный блок «Прикреплённые заметки».
3. `POST /api/{task,note}/:id/{notes,tasks}/:{nid,tid}`.
4. Клик по элементу в блоке → переход на его страницу.

### Поиск
- Поле в sidebar. `Enter` → `#/search?q=...`.
- Серверный поиск через `LIKE '%q%'` по `title` и `body_md` (для заметок) / `title` (для задач).
- Результаты — единый список, помеченный типом.

## Обработка ошибок

**Бэкенд:**
- HTTP-коды: 400 (валидация), 401 (нет токена), 403 (нет доступа), 404, 409 (конфликт), 500.
- Middleware `recover` для паник.
- Каждый запрос получает `request_id`, логируется.
- Валидация входных данных в начале каждого handler.

**Фронт:**
- Глобальный обработчик `fetch`: при `!ok` — toast с сообщением.
- При 401 → logout + редирект.
- Сетевые ошибки → toast «Нет соединения».
- Пустые состояния — placeholder с подсказкой.

## Тестирование

**Бэкенд (Go, `go test`):**
- `internal/db/users_test.go` — CRUD, дубликаты username.
- `internal/db/tasks_test.go` — CRUD, фильтры, шаринг, доступ.
- `internal/db/notes_test.go` — CRUD, версионирование, восстановление, ссылки.
- `internal/auth/auth_test.go` — хеширование, генерация/валидация JWT.
- `internal/handlers/*_test.go` — табличные тесты: 400, 401, 403, 404, happy path.
- БД в тестах — in-memory SQLite (`:memory:`).

**Фронтенд:**
- Ручные smoke-тесты по чек-листу в `README.md` (регистрация → создание задачи → создание заметки → шаринг → версионирование).
- Markdown-рендеринг: `web/test-markdown.html` с примерами для визуальной проверки.

## Запуск

```bash
cd ~/IdeaProjects/todoshka
go mod tidy
go run .
# открыть http://localhost:8080
```

## Что НЕ входит в первую версию (явный YAGNI)

- Email-подтверждение регистрации
- Real-time обновления (WebSockets/SSE) — данные обновляются по запросу
- Полнотекстовый поиск (FTS5) — только `LIKE`
- Загрузка файлов и вложения
- Тёмная тема (добавим позже, если попросят)
- Мобильное приложение — только адаптивная вёрстка
- Уведомления (email/push)
- Экспорт/импорт данных

## Решения по именованию и соглашениям

- Имя проекта: `todoshka`
- API-маршруты: kebab-case не используется, всё в нижнем регистре (`/api/notes/:id`)
- Имена таблиц: snake_case, множественное число (`task_tags`, `note_versions`)
- Имена колонок: snake_case
- ID: INTEGER, autoincrement
- JSON-поля в ответах API: snake_case (соответствует БД)
- Ошибки: `code` в верхнем регистре с подчёркиванием (`VALIDATION_FAILED`, `NOT_FOUND`, `UNAUTHORIZED`)
