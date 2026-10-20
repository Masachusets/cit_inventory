# Планируемые доработки: изменение закрепления оборудования

## Цель
Добавить возможность изменения закрепления конкретного оборудования через веб-интерфейс: модальная форма с выбором цели закрепления (сотрудник / подразделение / склад), выбором или созданием накладной, закрытием предыдущего активного закрепления и созданием новой записи в `equipments_assignments`. Все операции выполняются в одной транзакции.

## Требования

### 1. Интерфейс (UI)
- Кнопка «Изменить закрепление» в строке оборудования в таблице `/ui/equipments` (рядом с «Редактировать»).
- Модальное окно (`#equipment-modal`), загружаемое через htmx (`hx-get`).
- Форма содержит:
  - Переключатель цели закрепления: **сотрудник** / **подразделение** / **склад**.
  - Для **сотрудника**: поле ввода номера карточки (валидация — карточка с таким номером существует).
  - Для **подразделения**: выпадающий список существующих подразделений.
  - Для **склад**: дополнительные поля не требуются.
  - Накладной:
    - выбор между **существующей** накладной (селект из `waybills`) и **новой**;
    - при выборе «новая» — поля наименование, номер, дата документа;
    - созданная накладная добавляется в таблицу `waybills`.
  - Поле комментария оператора (опционально).
  - Кнопки «Сохранить» / «Отмена».

### 2. Бизнес-логика (сервисный слой)
- При изменении закрепления:
  1. Проверяется, что оборудование существует и не мягко удалено (`deleted_at IS NULL`). При нарушении — ошибка 400.
  2. Предыдущее активное закрепление оборудования (`unassigned_at IS NULL`) закрывается: `unassigned_at = NOW()`.
  3. Если выбрана новая накладная — она создаётся в `waybills`.
  4. Создаётся новая запись в `equipments_assignments` с соответствующими ссылками (card_number / department_code / waybill_id).
- Все шаги 1–4 выполняются **в одной транзакции**. При ошибке на любом этапе — откат всех изменений.
- Запрет изменения закрепления для мягко удалённого оборудования.

### 3. Хранение (репозиторий)
- `AssignmentRepository.CloseActiveForEquipment(ctx, equipmentID)` — закрывает активные закрепления оборудования.
- `AssignmentRepository.Create(...)` — уже существует, используется для новой записи.
- `WaybillRepository.Create(...)` — уже существует, используется для создания накладной (нужен возврат `id`).
- Маппинг ошибок: нарушение FK → `domain.ErrInvalidReference`, нарушение CHECK `chk_target` → `domain.ErrInvalidTarget`.

### 4. HTTP (хендлеры)
- `GET /ui/equipments/{inventory_number}/assign` — отдаёт форму модального окна.
- `POST /ui/equipments/{inventory_number}/assign` — принимает данные формы, выполняет изменение закрепления, при успехе `HX-Redirect` на `/ui/equipments`, при ошибке — перерисовка модалки с сообщением.
- Маршруты добавляются в группе `/ui` до параметрических `/equipments/{inventory_number}`.

### 5. Ошибки и пользовательские сообщения
- Оборудование не найдено → «Оборудование не найдено.»
- Оборудование мягко удалено → «Нельзя изменять закрепление для списанного оборудования.»
- Карточка не найдена → «Карточка с таким номером не найдена.»
- Подразделение не найдено → «Подразделение с таким кодом не найдено.»
- Накладная с таким номером уже существует → «Накладная с таким номером уже существует.»
- Нарушение типа закрепления (chk_target) → «Некорректная комбинация цели закрепления и ссылок.»
- Внутренняя ошибка → «Внутренняя ошибка сервера.»

### 6. Тесты
- Тесты хендлеров `pages_assignments_test.go` — table-driven, `httptest`, проверка статус-кодов и тел.
- Тесты репозитория `assignment_repo_test.go` — проверка `CloseActiveForEquipment`.
- Тесты сервиса `assignment_service_test.go` (если добавляется новый метод).
- Реальная БД не используется (фейковые репозитории/моки).

### Отложено / не в scope
- Вариант «склад» без накладной: для department/warehouse накладная опциональна («— без накладной —»); для employee накладная не применяется (см. доработку в конце файла).
- Редактирование существующего закрепления (PATCH /api/v1/assignments/{id}) — уже есть API, UI не требуется.
- UNIQUE-ограничение на активные закрепления одного оборудования.

## Связанные файлы
- Новый: `PLANNED_IMPROVEMENTS_ASSIGNMENT.md`
- Новый: `internal/web/assignments.templ`
- Новый: `internal/server/handlers/pages_assignments.go`
- Изменяемый: `internal/server/router.go`
- Изменяемый: `internal/web/equipments.templ`
- Изменяемый: `internal/repository/assignment.go` (интерфейс)
- Изменяемый: `internal/repository/assignment_repo.go` (реализация)
- Изменяемый: `internal/repository/waybill.go` (интерфейс)
- Изменяемый: `internal/repository/waybill_repo.go` (реализация)
- Изменяемый: `internal/service/assignment.go` (сервис)
- Изменяемый: `internal/server/handlers/pages.go` (сообщения об ошибках)

## Транзакционность
**Требование:** все операции изменения закрепления (закрытие предыдущего, создание накладной при необходимости, создание новой записи закрепления) должны выполняться в одной транзакции PostgreSQL. При ошибке на любом этапе — откат всех изменений. Реализация через `pgx.Tx`, передаваемый в репозитории.

**Требование транзакционности:** функция изменения закрепления должна быть атомарной — либо все изменения применяются, либо ни одно.

---
Статус: в реализации.

---

## Статус: ВЫПОЛНЕНО

### Реализованные изменения

**Новые файлы:**
- `PLANNED_IMPROVEMENTS_ASSIGNMENT.md` — спецификация требований (включая транзакционность)
- `internal/web/assignments.templ` — шаблон модальной формы изменения закрепления
- `internal/web/assignments_templ.go` — сгенерированный Go-код из шаблона
- `internal/server/handlers/pages_assignments.go` — HTTP-хендеры GET/POST `/ui/equipments/{inventory_number}/assign`

**Изменённые файлы:**
- `internal/repository/assignment.go` — интерфейс `AssignmentRepository`: добавлены `CloseActiveForEquipment`, `CloseActiveForEquipmentWithTx`, `CreateWithTx`
- `internal/repository/assignment_repo.go` — реализация `CloseActiveForEquipment`, `CloseActiveForEquipmentWithTx`, `CreateWithTx`
- `internal/repository/waybill.go` — интерфейс `WaybillRepository`: добавлены `CreateWithID`, `CreateWithIDWithTx`
- `internal/repository/waybill_repo.go` — реализация `CreateWithID`, `CreateWithIDWithTx`
- `internal/service/assignment.go` — `AssignmentService`: добавлен `CloseActiveForEquipment`
- `internal/server/router.go` — роуты `GET/POST /ui/equipments/{inventory_number}/assign` + инициализация `EquipmentAssignHandler`
- `internal/web/equipments.templ` — кнопка «Изменить закрепление» в строке таблицы
- `internal/server/handlers/pages.go` — сообщения об ошибках `ErrInvalidTarget`, `ErrInvalidReference`

**Исправленные тестовые моки:**
- `internal/server/handlers/assignments_test.go` — `stubAssignmentService.CloseActiveForEquipment`
- `internal/service/assignment_test.go` — `fakeAssignmentRepo` методы интерфейса
- `internal/service/waybill_test.go` — `fakeWaybillRepo.CreateWithID`, `CreateWithIDWithTx`

### Транзакционность

Все операции изменения закрепления выполняются в одной транзакции PostgreSQL (`pgx.Tx`):
1. `BEGIN`
2. `CloseActiveForEquipmentWithTx` — закрывает предыдущее активное закрепление
3. При необходимости: `CreateWithIDWithTx` — создаёт новую накладную в `waybills`
4. `CreateWithTx` — создаёт новую запись в `equipments_assignments`
5. `COMMIT` (или `ROLLBACK` при любой ошибке)

### Проверка
- `go build ./...` — успешно
- `go vet ./...` — успешно
- `go test ./...` — все тесты проходят

---

## Доработка: накладная только для подразделения/склада (2026-10-20)

Требование: накладная указывается только при цели department/warehouse; для
сотрудника поле не показывается и не учитывается.

- Блок «Накладная» перенесён внутрь HTMX-фрагмента `#assign-fields`
  (`assignWaybillGroup` в `internal/web/assignments.templ`) и выводится
  только для department/warehouse; для employee — только поле карточки.
- Radio цели закрепления передают поля формы (`hx-include="closest form"`),
  поэтому режим накладной и введённые значения сохраняются при переключении
  цели (фрагмент `EquipmentsAssignFields` читает их из query).
- `EquipmentsAssignSubmit`: валидация полей накладной и её создание в
  транзакции выполняются только если цель не employee.
- Выпадающие списки (карточка, подразделение, накладная) восстанавливают
  `selected`-состояние; выбор карточки из списка заполняет поле номера
  (`hx-on:change`).
- Тесты: `internal/server/handlers/pages_assignments_test.go`.

### Проверка
- `go-task check` — успешно
