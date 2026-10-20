package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/go-chi/chi/v5"
)

// newAssignHandlerForTest создаёт EquipmentAssignHandler со stub-сервисами
// справочников; repo и pool не используются в GET-эндпоинтах фрагментов.
func newAssignHandlerForTest(
	equipments *stubEquipmentService,
	cards *stubCardService,
	departments *stubDepartmentService,
	waybills *stubWaybillService,
) *EquipmentAssignHandler {
	if equipments == nil {
		equipments = &stubEquipmentService{}
	}
	if cards == nil {
		cards = &stubCardService{}
	}
	if departments == nil {
		departments = &stubDepartmentService{}
	}
	if waybills == nil {
		waybills = &stubWaybillService{}
	}
	return NewEquipmentAssignHandler(
		nil, equipments, cards, departments, waybills, nil, nil, nil,
		slog.New(slog.DiscardHandler),
	)
}

// reqWithInventoryNumber внедряет chi route context с path-параметром
// {inventory_number}.
func reqWithInventoryNumber(req *http.Request, number string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("inventory_number", number)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// TestEquipmentAssign_FormEmployeeNoWaybill проверяет, что модальная форма
// по умолчанию (сотрудник) не содержит блока накладной и что radio цели
// передают поля формы через hx-include.
func TestEquipmentAssign_FormEmployeeNoWaybill(t *testing.T) {
	h := newAssignHandlerForTest(&stubEquipmentService{getByInv: activeEquipment()}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments/ИТ00042/assign", nil)
	req = reqWithInventoryNumber(req, "ИТ00042")
	rr := httptest.NewRecorder()
	h.EquipmentsAssign(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Цель закрепления",
		`hx-include="closest form"`,
		"Номер карточки",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("тело не содержит %q", want)
		}
	}
	for _, absent := range []string{"waybill_mode", "waybill-fields", "Накладная"} {
		if strings.Contains(body, absent) {
			t.Errorf("тело не должно содержать %q (цель — сотрудник)", absent)
		}
	}
}

// TestEquipmentsAssignFields проверяет фрагмент полей цели закрепления:
// накладная выводится только для department/warehouse, режим new отдаёт
// поля ввода, некорректная цель — 400.
func TestEquipmentsAssignFields(t *testing.T) {
	card := &domain.Card{Number: 101, FullName: "Иванов Иван Иванович", IsActive: true}
	dept := &domain.Department{Code: 301, Name: "опк Гомель", IsActive: true}
	wb := waybill123()

	tests := []struct {
		name        string
		query       string
		cards       *stubCardService
		departments *stubDepartmentService
		waybills    *stubWaybillService
		wantStatus  int
		contains    []string
		absent      []string
	}{
		{
			name:       "сотрудник: карточки, без блока накладной",
			query:      "?target_type=employee",
			cards:      &stubCardService{listActive: []*domain.Card{card}},
			wantStatus: http.StatusOK,
			contains:   []string{"Номер карточки", "Иванов Иван Иванович"},
			absent:     []string{"waybill_mode", "waybill-fields", "Накладная"},
		},
		{
			name:        "подразделение: выбор + блок накладной",
			query:       "?target_type=department",
			departments: &stubDepartmentService{listActive: []*domain.Department{dept}},
			waybills:    &stubWaybillService{listItems: []*domain.Waybill{wb}},
			wantStatus:  http.StatusOK,
			contains: []string{
				"опк Гомель",
				"name=\"waybill_mode\"",
				"name=\"waybill_id\"",
				"Путевой лист",
			},
			absent: []string{"Номер карточки"},
		},
		{
			name:       "склад: текст без полей + блок накладной",
			query:      "?target_type=warehouse",
			waybills:   &stubWaybillService{listItems: []*domain.Waybill{wb}},
			wantStatus: http.StatusOK,
			contains:   []string{"Закрепление на складе", "name=\"waybill_mode\"", "Путевой лист"},
		},
		{
			name:       "режим новой накладной: поля ввода вместо селекта",
			query:      "?target_type=department&waybill_mode=new&waybill_name=" + url.QueryEscape("Путевой лист"),
			waybills:   &stubWaybillService{},
			wantStatus: http.StatusOK,
			contains:   []string{"name=\"waybill_name\"", "Путевой лист", "name=\"waybill_number\""},
			absent:     []string{"name=\"waybill_id\""},
		},
		{
			name:       "некорректная цель: 400",
			query:      "?target_type=alien",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAssignHandlerForTest(nil, tt.cards, tt.departments, tt.waybills)

			req := httptest.NewRequest(http.MethodGet, "/ui/equipments/ИТ00042/assign/fields"+tt.query, nil)
			req = reqWithInventoryNumber(req, "ИТ00042")
			rr := httptest.NewRecorder()
			h.EquipmentsAssignFields(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			body := rr.Body.String()
			for _, want := range tt.contains {
				if !strings.Contains(body, want) {
					t.Errorf("тело не содержит %q", want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(body, absent) {
					t.Errorf("тело не должно содержать %q", absent)
				}
			}
		})
	}
}
