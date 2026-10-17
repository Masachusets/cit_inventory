package handlers

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// newPageHandlerForTest создаёт PageHandler со stub-сервисами: по умолчанию
// возвращают пустые списки; нужные значения задаются в тесте.
func newPageHandlerForTest(
	equipments *stubEquipmentService,
	cards *stubCardService,
	departments *stubDepartmentService,
	nomenclatures *stubNomenclatureService,
	waybills *stubWaybillService,
) *PageHandler {
	if equipments == nil {
		equipments = &stubEquipmentService{}
	}
	if cards == nil {
		cards = &stubCardService{}
	}
	if departments == nil {
		departments = &stubDepartmentService{}
	}
	if nomenclatures == nil {
		nomenclatures = &stubNomenclatureService{}
	}
	if waybills == nil {
		waybills = &stubWaybillService{}
	}
	return NewPageHandler(equipments, cards, departments, nomenclatures, waybills, slog.New(slog.DiscardHandler))
}

// TestPageHandler_Equipments_FullPage проверяет полную страницу списка
// оборудования: layout, таблица, данные из сервиса.
func TestPageHandler_Equipments_FullPage(t *testing.T) {
	svc := &stubEquipmentService{listActive: []*domain.Equipment{activeEquipment()}}
	h := newPageHandlerForTest(svc, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments", nil)
	rr := httptest.NewRecorder()
	h.Equipments(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"<title>Оборудование · CIT Inventory</title>",
		`id="equipment-filter"`,
		`hx-get="/ui/equipments"`,
		"ИТ00042",              // инвентарный номер из данных
		"Ноутбук Clevo 15",     // номенклатура (JOIN)
		"Иванов Иван Иванович", // закрепление (JOIN)
		"Всего записей: 1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("тело не содержит %q", want)
		}
	}
}

// TestPageHandler_Equipments_HTMXFragment проверяет, что запрос от htmx
// (HX-Request: true) получает только фрагмент #equipment-table без layout.
func TestPageHandler_Equipments_HTMXFragment(t *testing.T) {
	svc := &stubEquipmentService{listActive: []*domain.Equipment{activeEquipment()}}
	h := newPageHandlerForTest(svc, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	h.Equipments(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if strings.Contains(body, "<!doctype html>") {
		t.Error("htmx-ответ не должен содержать полный HTML-документ")
	}
	if !strings.Contains(body, `id="equipment-table"`) {
		t.Error("htmx-ответ не содержит контейнер #equipment-table")
	}
	if !strings.Contains(body, "ИТ00042") {
		t.Error("htmx-ответ не содержит данные")
	}
}

// TestPageHandler_Equipments_FilterTranslation проверяет трансляцию query
// в доменный фильтр: строки, статус, мусорный статус игнорируется.
func TestPageHandler_Equipments_FilterTranslation(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		check   func(t *testing.T, f domain.EquipmentFilter)
		wantNil bool
	}{
		{
			name:  "строковые поля и статус",
			query: "/ui/equipments?inventory_number=ИТ00&model_name=clevo&location=кабинет&card_full_name=Иванов&department_name=опк&status=broken",
			check: func(t *testing.T, f domain.EquipmentFilter) {
				t.Helper()
				if f.InventoryNumber == nil || *f.InventoryNumber != "ИТ00" {
					t.Errorf("InventoryNumber = %v, want ИТ00", f.InventoryNumber)
				}
				if f.ModelName == nil || *f.ModelName != "clevo" {
					t.Errorf("ModelName = %v, want clevo", f.ModelName)
				}
				if f.Location == nil || *f.Location != "кабинет" {
					t.Errorf("Location = %v, want кабинет", f.Location)
				}
				if f.CardFullName == nil || *f.CardFullName != "Иванов" {
					t.Errorf("CardFullName = %v, want Иванов", f.CardFullName)
				}
				if f.DepartmentName == nil || *f.DepartmentName != "опк" {
					t.Errorf("DepartmentName = %v, want опк", f.DepartmentName)
				}
				if f.Status == nil || *f.Status != domain.StatusBroken {
					t.Errorf("Status = %v, want broken", f.Status)
				}
			},
		},
		{
			name:  "пустой запрос — пустой фильтр",
			query: "/ui/equipments",
			check: func(t *testing.T, f domain.EquipmentFilter) {
				t.Helper()
				if f.InventoryNumber != nil || f.ModelName != nil || f.Location != nil ||
					f.CardFullName != nil || f.DepartmentName != nil || f.Status != nil {
					t.Errorf("ожидался пустой фильтр, получено %+v", f)
				}
			},
		},
		{
			name:  "мусорный статус игнорируется",
			query: "/ui/equipments?status=deadbeef",
			check: func(t *testing.T, f domain.EquipmentFilter) {
				t.Helper()
				if f.Status != nil {
					t.Errorf("Status = %v, want nil (мусор игнорируется)", f.Status)
				}
			},
		},
		{
			name:  "пробелы обрезаются",
			query: "/ui/equipments?inventory_number=%20%D0%98%D0%A200042%20",
			check: func(t *testing.T, f domain.EquipmentFilter) {
				t.Helper()
				if f.InventoryNumber == nil || *f.InventoryNumber != "ИТ00042" {
					t.Errorf("InventoryNumber = %v, want ИТ00042 без пробелов", f.InventoryNumber)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubEquipmentService{listActive: []*domain.Equipment{}}
			h := newPageHandlerForTest(svc, nil, nil, nil, nil)

			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			rr := httptest.NewRecorder()
			h.Equipments(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			tt.check(t, svc.lastFilter)
		})
	}
}

// TestPageHandler_Equipments_ServiceError проверяет ответ 500 при ошибке
// сервиса (без утечки деталей ошибки в тело).
func TestPageHandler_Equipments_ServiceError(t *testing.T) {
	svc := &stubEquipmentService{listErr: domain.ErrNotFound}
	h := newPageHandlerForTest(svc, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments", nil)
	rr := httptest.NewRecorder()
	h.Equipments(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rr.Body.String(), "not found") {
		t.Error("в тело ответа утекло описание внутренней ошибки")
	}
}

// sectionHandler возвращает заполненный PageHandler со всеми stub-сервисами,
// обработчики и ожидаемые содержимые полных страниц разделов.
func sectionHandler(t *testing.T) (*PageHandler, map[string]http.HandlerFunc, map[string][]string) {
	t.Helper()

	card, err := domain.NewCard(101, "Иванов Иван Иванович")
	if err != nil {
		t.Fatalf("domain.NewCard: %v", err)
	}
	dept, err := domain.NewDepartment(100, "опк Гомель")
	if err != nil {
		t.Fatalf("domain.NewDepartment: %v", err)
	}
	when := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)

	h := newPageHandlerForTest(
		&stubEquipmentService{listActive: []*domain.Equipment{activeEquipment()}},
		&stubCardService{listActive: []*domain.Card{card}},
		&stubDepartmentService{listActive: []*domain.Department{dept}},
		&stubNomenclatureService{listItems: []*domain.Nomenclature{
			{ID: 1, Code: "08.01.02.00.00/0041", Name: "Ноутбук Clevo 15"},
		}},
		&stubWaybillService{listItems: []*domain.Waybill{
			{ID: 1, Name: "Товарная накладная", Number: strPtr("№12"), IssueDate: &when},
		}},
	)
	handlers := map[string]http.HandlerFunc{
		"equipments":    h.Equipments,
		"cards":         h.Cards,
		"departments":   h.Departments,
		"nomenclatures": h.Nomenclatures,
		"waybills":      h.Waybills,
	}
	wants := map[string][]string{
		"equipments": {
			"<title>Оборудование · CIT Inventory</title>",
			`id="equipment-filter"`, `id="equipment-table"`,
			"ИТ00042", "Ноутбук Clevo 15", "Всего записей: 1",
		},
		"cards": {
			"<title>Карточки · CIT Inventory</title>",
			`id="cards-filter"`, `id="cards-table"`,
			"Иванов Иван Иванович", "Всего записей: 1",
		},
		"departments": {
			"<title>Подразделения · CIT Inventory</title>",
			`id="departments-filter"`, `id="departments-table"`,
			"опк Гомель", "100", "Всего записей: 1",
		},
		"nomenclatures": {
			"<title>Номенклатура · CIT Inventory</title>",
			`id="nomenclatures-filter"`, `id="nomenclatures-table"`,
			"08.01.02.00.00/0041", "Ноутбук Clevo 15", "Всего записей: 1",
		},
		"waybills": {
			"<title>Накладные · CIT Inventory</title>",
			`id="waybills-filter"`, `id="waybills-table"`,
			"Товарная накладная", "№12", "12.05.2026", "Всего записей: 1",
		},
	}
	return h, handlers, wants
}

// TestPageHandler_SectionPages проверяет полные страницы всех разделов:
// заголовок, форма фильтра, контейнер таблицы и данные из сервисов.
func TestPageHandler_SectionPages(t *testing.T) {
	_, handlers, wants := sectionHandler(t)

	for section, handler := range handlers {
		t.Run(section, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ui/"+section, nil)
			rr := httptest.NewRecorder()
			handler(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("Content-Type = %q, want text/html", ct)
			}
			body := rr.Body.String()
			for _, want := range wants[section] {
				if !strings.Contains(body, want) {
					t.Errorf("тело не содержит %q", want)
				}
			}
		})
	}
}

// TestPageHandler_SectionHTMXFragments проверяет, что htmx-запросы разделов
// получают только фрагмент таблицы без полного HTML-документа.
func TestPageHandler_SectionHTMXFragments(t *testing.T) {
	_, handlers, _ := sectionHandler(t)
	containers := map[string]string{
		"equipments":    `id="equipment-table"`,
		"cards":         `id="cards-table"`,
		"departments":   `id="departments-table"`,
		"nomenclatures": `id="nomenclatures-table"`,
		"waybills":      `id="waybills-table"`,
	}

	for section, handler := range handlers {
		t.Run(section, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ui/"+section, nil)
			req.Header.Set("HX-Request", "true")
			rr := httptest.NewRecorder()
			handler(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			body := rr.Body.String()
			if strings.Contains(body, "<!doctype html>") {
				t.Error("htmx-ответ не должен содержать полный HTML-документ")
			}
			if !strings.Contains(body, containers[section]) {
				t.Errorf("htmx-ответ не содержит контейнер %s", containers[section])
			}
		})
	}
}

// TestPageHandler_SectionFilters проверяет трансляцию query-параметров
// разделов в доменные фильтры.
func TestPageHandler_SectionFilters(t *testing.T) {
	tests := []struct {
		name    string
		section string
		query   string
		check   func(t *testing.T, h *PageHandler)
	}{
		{
			name:    "cards: full_name",
			section: "cards",
			query:   "/ui/cards?full_name=%D0%98%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2",
			check: func(t *testing.T, h *PageHandler) {
				f := h.cards.(*stubCardService).lastFilter
				if f.FullName == nil || *f.FullName != "Иванов" {
					t.Errorf("FullName = %v, want Иванов", f.FullName)
				}
			},
		},
		{
			name:    "departments: name",
			section: "departments",
			query:   "/ui/departments?name=%D0%BE%D0%BF%D0%BA",
			check: func(t *testing.T, h *PageHandler) {
				f := h.departments.(*stubDepartmentService).lastFilter
				if f.Name == nil || *f.Name != "опк" {
					t.Errorf("Name = %v, want опк", f.Name)
				}
			},
		},
		{
			name:    "nomenclatures: code и name",
			section: "nomenclatures",
			query:   "/ui/nomenclatures?code=08.01&name=%D0%9D%D0%BE%D1%83%D1%82%D0%B1%D1%83%D0%BA",
			check: func(t *testing.T, h *PageHandler) {
				f := h.nomenclatures.(*stubNomenclatureService).lastFilter
				if f.Code == nil || *f.Code != "08.01" {
					t.Errorf("Code = %v, want 08.01", f.Code)
				}
				if f.Name == nil || *f.Name != "Ноутбук" {
					t.Errorf("Name = %v, want Ноутбук", f.Name)
				}
			},
		},
		{
			name:    "waybills: name и number",
			section: "waybills",
			query:   "/ui/waybills?name=%D0%A2%D0%BE%D0%B2%D0%B0%D1%80%D0%BD%D0%B0%D1%8F&number=12",
			check: func(t *testing.T, h *PageHandler) {
				f := h.waybills.(*stubWaybillService).lastFilter
				if f.Name == nil || *f.Name != "Товарная" {
					t.Errorf("Name = %v, want Товарная", f.Name)
				}
				if f.Number == nil || *f.Number != "12" {
					t.Errorf("Number = %v, want 12", f.Number)
				}
			},
		},
		{
			name:    "waybills: пустой запрос — пустой фильтр",
			section: "waybills",
			query:   "/ui/waybills",
			check: func(t *testing.T, h *PageHandler) {
				f := h.waybills.(*stubWaybillService).lastFilter
				if f.Name != nil || f.Number != nil {
					t.Errorf("ожидался пустой фильтр, получено %+v", f)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, handlers, _ := sectionHandler(t)

			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			rr := httptest.NewRecorder()
			handlers[tt.section](rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			tt.check(t, h)
		})
	}
}

// TestPageHandler_SectionServiceErrors проверяет ответ 500 при ошибках
// сервисов разделов (без утечки деталей в тело).
func TestPageHandler_SectionServiceErrors(t *testing.T) {
	tests := []struct {
		section string
		handler func(h *PageHandler) http.HandlerFunc
		svc     *PageHandler
	}{
		{
			section: "cards",
			handler: func(h *PageHandler) http.HandlerFunc { return h.Cards },
			svc:     newPageHandlerForTest(nil, &stubCardService{listErr: domain.ErrNotFound}, nil, nil, nil),
		},
		{
			section: "departments",
			handler: func(h *PageHandler) http.HandlerFunc { return h.Departments },
			svc:     newPageHandlerForTest(nil, nil, &stubDepartmentService{listErr: domain.ErrNotFound}, nil, nil),
		},
		{
			section: "nomenclatures",
			handler: func(h *PageHandler) http.HandlerFunc { return h.Nomenclatures },
			svc:     newPageHandlerForTest(nil, nil, nil, &stubNomenclatureService{listErr: domain.ErrNotFound}, nil),
		},
		{
			section: "waybills",
			handler: func(h *PageHandler) http.HandlerFunc { return h.Waybills },
			svc:     newPageHandlerForTest(nil, nil, nil, nil, &stubWaybillService{listErr: domain.ErrNotFound}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.section, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ui/"+tt.section, nil)
			rr := httptest.NewRecorder()
			tt.handler(tt.svc)(rr, req)

			if rr.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
			}
			if strings.Contains(rr.Body.String(), "not found") {
				t.Error("в тело ответа утекло описание внутренней ошибки")
			}
		})
	}
}
