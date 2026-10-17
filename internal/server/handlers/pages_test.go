package handlers

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// newPageHandlerForTest создаёт PageHandler со stub-сервисом оборудования.
func newPageHandlerForTest(svc *stubEquipmentService) *PageHandler {
	return NewPageHandler(svc, slog.New(slog.DiscardHandler))
}

// TestPageHandler_Equipments_FullPage проверяет полную страницу списка
// оборудования: layout, таблица, данные из сервиса.
func TestPageHandler_Equipments_FullPage(t *testing.T) {
	svc := &stubEquipmentService{listActive: []*domain.Equipment{activeEquipment()}}
	h := newPageHandlerForTest(svc)

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
	h := newPageHandlerForTest(svc)

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
			h := newPageHandlerForTest(svc)

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
	h := newPageHandlerForTest(svc)

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
