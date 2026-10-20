package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Masachusets/cit_inventory/internal/repository"
	"github.com/Masachusets/cit_inventory/internal/server/handlers"
	"github.com/Masachusets/cit_inventory/internal/server/middleware"
	"github.com/Masachusets/cit_inventory/internal/service"
	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(log *slog.Logger, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(middleware.Logger(log))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	health := handlers.NewHealthHandler(pool)
	r.Get("/health/live", health.Live)
	r.Get("/health/ready", health.Ready)

	// Корень перенаправляет в веб-интерфейс; статика (htmx, CSS) — из go:embed.
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/ui", http.StatusFound)
	})
	r.Handle("/static/*", web.StaticHandler())

	// Бизнес-слой: собираем цепочку зависимостей (composition root).
	// pool может быть nil в тестах — репозиторий создаётся лениво, обращения
	// к БД в health-проверках и роутинге не происходят.
	cardRepo := repository.NewCardRepo(pool)
	cardSvc := service.NewCardService(cardRepo)
	cards := handlers.NewCardHandler(cardSvc, log)

	nomenclatureRepo := repository.NewNomenclatureRepo(pool)
	nomenclatureSvc := service.NewNomenclatureService(nomenclatureRepo)
	nomenclatures := handlers.NewNomenclatureHandler(nomenclatureSvc, log)

	departmentRepo := repository.NewDepartmentRepo(pool)
	departmentSvc := service.NewDepartmentService(departmentRepo)
	departments := handlers.NewDepartmentHandler(departmentSvc, log)

	waybillRepo := repository.NewWaybillRepo(pool)
	waybillSvc := service.NewWaybillService(waybillRepo)
	waybills := handlers.NewWaybillHandler(waybillSvc, log)

	equipmentRepo := repository.NewEquipmentRepo(pool)
	equipmentSvc := service.NewEquipmentService(equipmentRepo)
	equipments := handlers.NewEquipmentHandler(equipmentSvc, log)

	assignmentRepo := repository.NewAssignmentRepo(pool)
	assignmentSvc := service.NewAssignmentService(assignmentRepo)
	assignments := handlers.NewAssignmentHandler(assignmentSvc, log)

	// Веб-интерфейс: серверный рендеринг (templ + htmx). Внутри группы —
	// свой NotFound (HTML вместо JSON API). Страницы разделов зависят от
	// сервисов, созданных выше в composition root.
	pages := handlers.NewPageHandler(equipmentSvc, cardSvc, departmentSvc, nomenclatureSvc, waybillSvc, assignmentSvc, log)
	equipAssign := handlers.NewEquipmentAssignHandler(
		assignmentSvc, equipmentSvc, cardSvc, departmentSvc, waybillSvc,
		assignmentRepo, waybillRepo, pool, log,
	)
	r.Route("/ui", func(r chi.Router) {
		r.Get("/", pages.Home)
		r.Get("/equipments", pages.Equipments)

		// Оборудование. Статические маршруты create/edit/assign объявлены ДО параметрических,
		// чтобы chi не перехватил "create"/"edit"/"assign" как {inventory_number}.
		r.Get("/equipments/create", pages.EquipmentsCreate)
		r.Post("/equipments/create", pages.EquipmentsCreateSubmit)
		r.Get("/equipments/{inventory_number}/edit", pages.EquipmentsEdit)
		r.Post("/equipments/{inventory_number}", pages.EquipmentsUpdateSubmit)
		r.Get("/equipments/{inventory_number}/assign", equipAssign.EquipmentsAssign)
		r.Post("/equipments/{inventory_number}/assign", equipAssign.EquipmentsAssignSubmit)

		// Частичные фрагменты формы закрепления (htmx подменяет поля
		// #assign-fields / #waybill-fields при переключении radio).
		r.Get("/equipments/{inventory_number}/assign/fields", equipAssign.EquipmentsAssignFields)
		r.Get("/equipments/{inventory_number}/assign/waybill-fields", equipAssign.EquipmentsAssignWaybillFields)

		// Карточки. Статические маршруты create/edit объявлены ДО параметрических
		// /cards/{number}, иначе chi перехватит "create"/"edit" как {number}.
		r.Get("/cards/create", pages.CardsCreate)
		r.Post("/cards/create", pages.CardsCreateSubmit)
		r.Get("/cards/{number}/edit", pages.CardsEdit)
		r.Post("/cards/{number}", pages.CardsUpdateSubmit)
		r.Get("/cards", pages.Cards)

		// Подразделения. Создание подразделения доступно только через API
		// (/api/v1/departments), в веб-интерфейсе доступно только редактирование.
		r.Get("/departments/{code}/edit", pages.DepartmentsEdit)
		r.Post("/departments/{code}", pages.DepartmentsUpdateSubmit)
		r.Get("/departments", pages.Departments)

		r.Get("/nomenclatures", pages.Nomenclatures)

		// Накладные. Статические маршруты create/edit ДО /waybills/{id}.
		r.Get("/waybills/create", pages.WaybillsCreate)
		r.Post("/waybills/create", pages.WaybillsCreateSubmit)
		r.Get("/waybills/{id}/edit", pages.WaybillsEdit)
		r.Post("/waybills/{id}", pages.WaybillsUpdateSubmit)
		r.Get("/waybills", pages.Waybills)

		r.NotFound(pages.PageNotFound)
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Статические маршруты /cards/inactive и /departments/inactive объявлены
		// ДО параметрических /{number} и /{code}, иначе chi перехватил бы
		// "inactive" как path-параметр.
		r.Get("/cards/inactive", cards.ListInactive)
		r.Get("/cards", cards.List)
		r.Post("/cards", cards.Create)

		r.Get("/cards/{number}", cards.Get)
		r.Patch("/cards/{number}", cards.Update)
		r.Post("/cards/{number}/deactivate", cards.Deactivate)
		r.Delete("/cards/{number}", cards.Delete)

		r.Get("/nomenclatures", nomenclatures.List)
		r.Get("/nomenclatures/{id}", nomenclatures.Get)

		r.Get("/departments/inactive", departments.ListInactive)
		r.Get("/departments", departments.List)
		r.Post("/departments", departments.Create)

		r.Get("/departments/{code}", departments.Get)
		r.Patch("/departments/{code}", departments.Update)
		r.Post("/departments/{code}/deactivate", departments.Deactivate)

		r.Get("/waybills", waybills.List)
		r.Post("/waybills", waybills.Create)

		r.Get("/waybills/{id}", waybills.Get)
		r.Patch("/waybills/{id}", waybills.Update)

		// Оборудование. Статические маршруты /equipments/inactive и
		// /equipments/inventory/{inventory_number} объявлены ДО параметрических
		// /equipments/{id}, иначе chi перехватил бы «inactive»/«inventory»
		// как path-параметр.
		r.Get("/equipments/inactive", equipments.ListInactive)
		r.Get("/equipments/inventory/{inventory_number}", equipments.GetByInventoryNumber)
		r.Patch("/equipments/inventory/{inventory_number}", equipments.UpdateByInventoryNumber)
		r.Delete("/equipments/inventory/{inventory_number}", equipments.DeleteByInventoryNumber)

		r.Get("/equipments", equipments.List)
		r.Post("/equipments", equipments.Create)

		r.Get("/equipments/{id}", equipments.GetByID)
		r.Patch("/equipments/{id}", equipments.UpdateByID)
		r.Delete("/equipments/{id}", equipments.DeleteByID)

		// Закрепления оборудования (история перемещений).
		r.Get("/assignments", assignments.List)
		r.Post("/assignments", assignments.Create)

		r.Get("/assignments/{id}", assignments.Get)
		r.Patch("/assignments/{id}", assignments.Update)
		r.Delete("/assignments/{id}", assignments.Delete)
	})

	r.NotFound(handlers.NotFound)
	r.MethodNotAllowed(handlers.MethodNotAllowed)

	return r
}
