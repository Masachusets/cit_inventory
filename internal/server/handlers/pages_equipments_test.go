package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// equipmentFormValues — значения формы оборудования (url-encoded).
func equipmentFormValues(pairs map[string]string) string {
	form := url.Values{}
	for k, v := range pairs {
		form.Set(k, v)
	}
	return form.Encode()
}

// timePtr — локальный хелпер указателя на time.Time.
func timePtr(v time.Time) *time.Time { return &v }

// equipmentReq — POST-запрос с формой оборудования и path-параметром
// {inventory_number} (при прямом вызове хендлера URLParam() пуст).
func equipmentReq(t *testing.T, target, inventoryNumber, form string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if inventoryNumber != "" {
		req = reqWithEquipmentParam(req, "inventory_number", inventoryNumber)
	}
	return req
}

// validEquipmentForm — корректно заполненная форма оборудования.
func validEquipmentForm() string {
	return equipmentFormValues(map[string]string{
		"inventory_number": "ИТ00042",
		"nomenclature_id":  "1",
		"model_name":       "Clevo",
		"serial_number":    "SN-1",
		"manufacture_date": "2015-03-10",
		"arrival_date":     "2012-08-01",
		"status":           "broken",
		"form_number":      "Ф-1",
		"location":         "каб. 101",
		"notes":            "прим.",
	})
}

// TestPageHandler_EquipmentsCreateModal проверяет GET /ui/equipments/create:
// отдаётся модальная форма создания со списком номенклатур и корректным
// action-URL, без полного HTML-документа.
func TestPageHandler_EquipmentsCreateModal(t *testing.T) {
	h := newPageHandlerForTest(nil, nil, nil, &stubNomenclatureService{
		listItems: []*domain.Nomenclature{laptop()},
	}, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments/create", nil)
	rr := httptest.NewRecorder()
	h.EquipmentsCreate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Новое оборудование",
		`hx-post="/ui/equipments/create"`,
		`hx-target="#equipment-modal"`,
		`name="inventory_number"`,
		`name="nomenclature_id"`,
		"08.01.02.00.00/0041 — Ноутбук Clevo 15",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("тело не содержит %q", want)
		}
	}
}

// TestPageHandler_EquipmentsEditModal проверяет GET /ui/equipments/{n}/edit:
// форма заполнена данными оборудования, инвентарный номер readonly.
func TestPageHandler_EquipmentsEditModal(t *testing.T) {
	eq := activeEquipment()
	eq.SerialNumber = strPtr("SN-1")
	eq.NomenclatureID = intPtr(1)
	eq.ModelName = strPtr("Clevo")
	eq.Location = strPtr("каб. 101")
	eq.ManufactureDate = timePtr(time.Date(2015, 3, 10, 0, 0, 0, 0, time.UTC))
	eq.ArrivalDate = timePtr(time.Date(2012, 8, 1, 0, 0, 0, 0, time.UTC))

	h := newPageHandlerForTest(
		&stubEquipmentService{getByInv: eq},
		nil, nil,
		&stubNomenclatureService{listItems: []*domain.Nomenclature{laptop()}},
		nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments/ИТ00042/edit", nil)
	req = reqWithEquipmentParam(req, "inventory_number", "ИТ00042")
	rr := httptest.NewRecorder()
	h.EquipmentsEdit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Редактирование ИТ00042",
		`hx-post="/ui/equipments/ИТ00042"`,
		`value="ИТ00042"`,
		`value="SN-1"`,
		`value="Clevo"`,
		`value="2015-03-10"`,
		`value="2012-08-01"`,
		`value="каб. 101"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("тело не содержит %q", want)
		}
	}
	if !strings.Contains(body, "readonly") {
		t.Error("в режиме редактирования инвентарный номер должен быть readonly")
	}
}

// TestPageHandler_EquipmentsEditModal_InvalidNumber проверяет ответ 400 при
// некорректном инвентарном номере в пути.
func TestPageHandler_EquipmentsEditModal_InvalidNumber(t *testing.T) {
	h := newPageHandlerForTest(nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments/не-номер/edit", nil)
	req = reqWithEquipmentParam(req, "inventory_number", "не-номер")
	rr := httptest.NewRecorder()
	h.EquipmentsEdit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestPageHandler_EquipmentsEditModal_NotFound проверяет, что при отсутствии
// оборудования показывается форма с сообщением «Запись не найдена».
func TestPageHandler_EquipmentsEditModal_NotFound(t *testing.T) {
	h := newPageHandlerForTest(
		&stubEquipmentService{getByInvErr: domain.ErrNotFound},
		nil, nil,
		&stubNomenclatureService{listItems: []*domain.Nomenclature{laptop()}},
		nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/ui/equipments/ИТ00042/edit", nil)
	req = reqWithEquipmentParam(req, "inventory_number", "ИТ00042")
	rr := httptest.NewRecorder()
	h.EquipmentsEdit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Запись не найдена.") {
		t.Errorf("тело не содержит сообщение об отсутствии записи")
	}
	if !strings.Contains(body, `hx-post="/ui/equipments/ИТ00042"`) {
		t.Errorf("тело не содержит action-URL формы редактирования")
	}
}

// TestPageHandler_EquipmentsCreateSubmit_Success проверяет успешное создание:
// оборудование создано, закреплено на складе, клиенту отдан редирект.
func TestPageHandler_EquipmentsCreateSubmit_Success(t *testing.T) {
	eqSvc := &stubEquipmentService{createResult: activeEquipment()}
	assignSvc := &stubAssignmentService{}
	h := newPageHandlerForTest(eqSvc, nil, nil, &stubNomenclatureService{
		listItems: []*domain.Nomenclature{laptop()},
	}, nil, assignSvc)

	req := equipmentReq(t, "/ui/equipments/create", "", validEquipmentForm())
	rr := httptest.NewRecorder()
	h.EquipmentsCreateSubmit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if loc := rr.Header().Get("HX-Redirect"); loc != "/ui/equipments" {
		t.Errorf("HX-Redirect = %q, want /ui/equipments", loc)
	}

	in := eqSvc.lastInput
	if in.InventoryNumber != "ИТ00042" {
		t.Errorf("InventoryNumber = %q, want ИТ00042", in.InventoryNumber)
	}
	if in.NomenclatureID == nil || *in.NomenclatureID != 1 {
		t.Errorf("NomenclatureID = %v, want 1", in.NomenclatureID)
	}
	if in.ManufactureDate == nil || in.ManufactureDate.Format("2006-01-02") != "2015-03-10" {
		t.Errorf("ManufactureDate = %v, want 2015-03-10", in.ManufactureDate)
	}
	if in.Status != "broken" {
		t.Errorf("Status = %q, want broken", in.Status)
	}

	assignIn := assignSvc.createInput
	if assignIn.EquipmentID != eqSvc.createResult.ID {
		t.Errorf("EquipmentID = %d, want %d", assignIn.EquipmentID, eqSvc.createResult.ID)
	}
	if assignIn.TargetType != string(domain.TargetWarehouse) {
		t.Errorf("TargetType = %q, want warehouse", assignIn.TargetType)
	}
}

// TestPageHandler_EquipmentsCreateSubmit_ValidationErrors проверяет, что при
// некорректных данных форма перерисовывается с сообщением об ошибке и
// оборудование не создаётся.
func TestPageHandler_EquipmentsCreateSubmit_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		form map[string]string
		want string
	}{
		{
			name: "некорректный инвентарный номер",
			form: map[string]string{"inventory_number": "ABC"},
			want: "Некорректный инвентарный номер",
		},
		{
			name: "не выбрана номенклатура",
			form: map[string]string{"inventory_number": "ИТ00042", "nomenclature_id": ""},
			want: "Выберите номенклатуру.",
		},
		{
			name: "некорректная дата выпуска",
			form: map[string]string{
				"inventory_number": "ИТ00042",
				"nomenclature_id":  "1",
				"manufacture_date": "10.03.2015",
			},
			want: "Некорректное значение поля.",
		},
		{
			name: "некорректный статус",
			form: map[string]string{
				"inventory_number": "ИТ00042",
				"nomenclature_id":  "1",
				"status":           "unknown",
			},
			want: "Некорректный статус оборудования.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqSvc := &stubEquipmentService{}
			h := newPageHandlerForTest(eqSvc, nil, nil, &stubNomenclatureService{
				listItems: []*domain.Nomenclature{laptop()},
			}, nil, nil)

			req := equipmentReq(t, "/ui/equipments/create", "", equipmentFormValues(tt.form))
			rr := httptest.NewRecorder()
			h.EquipmentsCreateSubmit(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			body := rr.Body.String()
			if !strings.Contains(body, tt.want) {
				t.Errorf("тело не содержит сообщение %q", tt.want)
			}
			if !strings.Contains(body, "Новое оборудование") {
				t.Error("тело не содержит форму создания")
			}
			if eqSvc.createResult != nil || eqSvc.createErr != nil {
				t.Error("оборудование не должно создаваться при некорректных данных")
			}
		})
	}
}

// TestPageHandler_EquipmentsCreateSubmit_ServiceError проверяет, что ошибка
// сервиса отображается в форме, а не приводит к 500.
func TestPageHandler_EquipmentsCreateSubmit_ServiceError(t *testing.T) {
	eqSvc := &stubEquipmentService{createErr: domain.ErrAlreadyExists}
	h := newPageHandlerForTest(eqSvc, nil, nil, &stubNomenclatureService{
		listItems: []*domain.Nomenclature{laptop()},
	}, nil, nil)

	req := equipmentReq(t, "/ui/equipments/create", "", validEquipmentForm())
	rr := httptest.NewRecorder()
	h.EquipmentsCreateSubmit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Запись с таким значением уже существует.") {
		t.Error("тело не содержит сообщение об ошибке сервиса")
	}
}

// TestPageHandler_EquipmentsUpdateSubmit_Success проверяет успешное обновление:
// данные переданы в сервис, клиенту отдан редирект.
func TestPageHandler_EquipmentsUpdateSubmit_Success(t *testing.T) {
	eqSvc := &stubEquipmentService{updateByInv: activeEquipment()}
	h := newPageHandlerForTest(eqSvc, nil, nil, &stubNomenclatureService{
		listItems: []*domain.Nomenclature{laptop()},
	}, nil, nil)

	req := equipmentReq(t, "/ui/equipments/ИТ00042", "ИТ00042", validEquipmentForm())
	rr := httptest.NewRecorder()
	h.EquipmentsUpdateSubmit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if loc := rr.Header().Get("HX-Redirect"); loc != "/ui/equipments" {
		t.Errorf("HX-Redirect = %q, want /ui/equipments", loc)
	}

	up := eqSvc.lastUpdate
	if up.NomenclatureID == nil || *up.NomenclatureID != 1 {
		t.Errorf("NomenclatureID = %v, want 1", up.NomenclatureID)
	}
	if up.ModelName == nil || *up.ModelName != "Clevo" {
		t.Errorf("ModelName = %v, want Clevo", up.ModelName)
	}
	if up.Status == nil || *up.Status != "broken" {
		t.Errorf("Status = %v, want broken", up.Status)
	}
}

// TestPageHandler_EquipmentsUpdateSubmit_InvalidNumber проверяет ответ 400 при
// некорректном инвентарном номере в пути.
func TestPageHandler_EquipmentsUpdateSubmit_InvalidNumber(t *testing.T) {
	h := newPageHandlerForTest(nil, nil, nil, nil, nil, nil)

	req := equipmentReq(t, "/ui/equipments/не-номер", "не-номер", validEquipmentForm())
	rr := httptest.NewRecorder()
	h.EquipmentsUpdateSubmit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestPageHandler_EquipmentsUpdateSubmit_InvalidDate проверяет, что при
// некорректной дате форма перерисовывается с сообщением об ошибке.
func TestPageHandler_EquipmentsUpdateSubmit_InvalidDate(t *testing.T) {
	eqSvc := &stubEquipmentService{}
	h := newPageHandlerForTest(eqSvc, nil, nil, &stubNomenclatureService{
		listItems: []*domain.Nomenclature{laptop()},
	}, nil, nil)

	form := equipmentFormValues(map[string]string{
		"inventory_number": "ИТ00042",
		"nomenclature_id":  "1",
		"arrival_date":     "01.08.2012",
	})
	req := equipmentReq(t, "/ui/equipments/ИТ00042", "ИТ00042", form)
	rr := httptest.NewRecorder()
	h.EquipmentsUpdateSubmit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Некорректное значение поля.") {
		t.Error("тело не содержит сообщение об ошибке")
	}
	if !strings.Contains(body, "Редактирование ИТ00042") {
		t.Error("тело не содержит форму редактирования")
	}
	if eqSvc.updateByInv != nil {
		t.Error("оборудование не должно обновляться при некорректных данных")
	}
}
