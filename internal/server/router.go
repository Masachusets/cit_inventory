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

	// Веб-интерфейс: серверный рендеринг (templ + htmx). Внутри группы —
	// свой NotFound (HTML вместо JSON API); бизнес-слой подключится позже,
	// когда появятся страницы разделов.
	pages := handlers.NewPageHandler(log)
	r.Route("/ui", func(r chi.Router) {
		r.Get("/", pages.Home)
		r.NotFound(pages.PageNotFound)
	})

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
