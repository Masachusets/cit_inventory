package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// stubAssignmentService — заглушка AssignmentService для тестов хендлеров без БД.
type stubAssignmentService struct {
	createResult *domain.Assignment
	createErr    error
	createInput  domain.AssignmentInput
	getResult    *domain.Assignment
	getErr       error
	listItems    []*domain.Assignment
	listErr      error
	updateResult *domain.Assignment
	updateErr    error
	updateLast   domain.AssignmentUpdate
	updateID     int
	deleteResult *domain.Assignment
	deleteErr    error
	lastFilter   domain.AssignmentFilter
}

func (s *stubAssignmentService) Create(_ context.Context, in domain.AssignmentInput) (*domain.Assignment, error) {
	s.createInput = in
	return s.createResult, s.createErr
}

func (s *stubAssignmentService) Get(_ context.Context, _ int) (*domain.Assignment, error) {
	return s.getResult, s.getErr
}

func (s *stubAssignmentService) List(_ context.Context, filter domain.AssignmentFilter) ([]*domain.Assignment, error) {
	s.lastFilter = filter
	return s.listItems, s.listErr
}

func (s *stubAssignmentService) Update(_ context.Context, id int, up domain.AssignmentUpdate) (*domain.Assignment, error) {
	s.updateID = id
	s.updateLast = up
	return s.updateResult, s.updateErr
}

func (s *stubAssignmentService) Delete(_ context.Context, _ int) (*domain.Assignment, error) {
	return s.deleteResult, s.deleteErr
}

func (s *stubAssignmentService) CloseActiveForEquipment(_ context.Context, _ int) error {
	return nil
}

// assignmentWithRelations — запись со всеми вложенными сущностями для проверки
// формата JSON-ответа.
func assignmentWithRelations() *domain.Assignment {
	num := "123/2026"
	date := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	comment := "установлен в каб. 214"
	return &domain.Assignment{
		ID:              1,
		EquipmentID:     10,
		TargetType:      domain.TargetEmployee,
		CardNumber:      ptrIntTest(101),
		WaybillID:       ptrIntTest(7),
		AssignedAt:      time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC),
		OperatorComment: &comment,
		Equipment:       &domain.Equipment{ID: 10, InventoryNumber: "ИТ00042"},
		Waybill:         &domain.Waybill{ID: 7, Name: "Путевой лист", Number: &num, IssueDate: &date},
		Card:            &domain.Card{Number: 101, FullName: "Иванов Иван Иванович"},
	}
}

// ptrIntTest — локальный хелпер указателя (имя уникально в пакете handlers).
func ptrIntTest(v int) *int { return &v }

func TestAssignmentHandler_Create(t *testing.T) {
	stub := &stubAssignmentService{createResult: assignmentWithRelations()}
	h := NewAssignmentHandler(stub, discardLogger())

	body := `{"equipment_id":10,"target_type":"employee","card_number":101,` +
		`"waybill_id":7,"assigned_at":"2026-10-15","operator_comment":"установлен в каб. 214"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assignments", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rr.Code, rr.Body.String())
	}
	in := stub.createInput
	if in.EquipmentID != 10 || in.TargetType != "employee" {
		t.Errorf("input = %+v", in)
	}
	if in.CardNumber == nil || *in.CardNumber != 101 || in.WaybillID == nil || *in.WaybillID != 7 {
		t.Errorf("ссылки = %+v", in)
	}
	if in.AssignedAt == nil || in.AssignedAt.Format("2006-01-02") != "2026-10-15" {
		t.Errorf("assigned_at = %v", in.AssignedAt)
	}
	if in.OperatorComment != "установлен в каб. 214" {
		t.Errorf("comment = %q", in.OperatorComment)
	}
}

func TestAssignmentHandler_Create_WarehouseDefaults(t *testing.T) {
	stub := &stubAssignmentService{createResult: assignmentWithRelations()}
	h := NewAssignmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assignments",
		bytes.NewBufferString(`{"equipment_id":10,"target_type":"warehouse"}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
	in := stub.createInput
	if in.AssignedAt != nil {
		t.Errorf("assigned_at = %v, want nil (DEFAULT NOW())", in.AssignedAt)
	}
	if in.CardNumber != nil || in.DepartmentCode != nil {
		t.Errorf("ссылки = %+v, want nil", in)
	}
}

func TestAssignmentHandler_Create_InvalidJSON(t *testing.T) {
	h := NewAssignmentHandler(&stubAssignmentService{}, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assignments", bytes.NewBufferString(`{`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestAssignmentHandler_Create_InvalidTimestamp(t *testing.T) {
	h := NewAssignmentHandler(&stubAssignmentService{}, discardLogger())

	body := `{"equipment_id":1,"target_type":"warehouse","assigned_at":"15.10.2026"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assignments", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestAssignmentHandler_Create_InvalidTarget(t *testing.T) {
	stub := &stubAssignmentService{createErr: domain.ErrInvalidTarget}
	h := NewAssignmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assignments",
		bytes.NewBufferString(`{"equipment_id":1,"target_type":"warehouse"}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestAssignmentHandler_Create_InvalidReference(t *testing.T) {
	stub := &stubAssignmentService{createErr: domain.ErrInvalidReference}
	h := NewAssignmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assignments",
		bytes.NewBufferString(`{"equipment_id":1,"target_type":"warehouse"}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestAssignmentHandler_List_FilterFromQuery(t *testing.T) {
	stub := &stubAssignmentService{}
	h := NewAssignmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/assignments?equipment_id=10&inventory_number=ИТ00042&target_type=employee"+
			"&card_number=101&card_full_name=Иванов&department_code=100&department_name=опк"+
			"&waybill_id=7&waybill_name=путевой&waybill_number=123&active=true"+
			"&operator_comment=кабинет"+
			"&waybill_issue_date_from=2026-10-01&waybill_issue_date_to=2026-10-31"+
			"&assigned_at_from=2026-09-01&assigned_at_to=2026-10-15"+
			"&unassigned_at_from=2026-10-01&unassigned_at_to=2026-10-31", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	f := stub.lastFilter
	if f.EquipmentID == nil || *f.EquipmentID != 10 {
		t.Errorf("equipment_id = %v", f.EquipmentID)
	}
	if f.InventoryNumber == nil || *f.InventoryNumber != "ИТ00042" {
		t.Errorf("inventory_number = %v", f.InventoryNumber)
	}
	if f.TargetType == nil || *f.TargetType != domain.TargetEmployee {
		t.Errorf("target_type = %v", f.TargetType)
	}
	if f.CardNumber == nil || *f.CardNumber != 101 || f.CardFullName == nil || *f.CardFullName != "Иванов" {
		t.Errorf("card-фильтры = %v / %v", f.CardNumber, f.CardFullName)
	}
	if f.DepartmentCode == nil || *f.DepartmentCode != 100 || f.DepartmentName == nil || *f.DepartmentName != "опк" {
		t.Errorf("department-фильтры = %v / %v", f.DepartmentCode, f.DepartmentName)
	}
	if f.WaybillID == nil || *f.WaybillID != 7 || f.WaybillName == nil || *f.WaybillName != "путевой" ||
		f.WaybillNumber == nil || *f.WaybillNumber != "123" {
		t.Errorf("waybill-фильтры = %v / %v / %v", f.WaybillID, f.WaybillName, f.WaybillNumber)
	}
	if f.Active == nil || !*f.Active {
		t.Errorf("active = %v, want true", f.Active)
	}
	if f.OperatorComment == nil || *f.OperatorComment != "кабинет" {
		t.Errorf("operator_comment = %v", f.OperatorComment)
	}
	if f.WaybillIssueDateFrom == nil || f.WaybillIssueDateTo == nil ||
		f.WaybillIssueDateFrom.Format("2006-01-02") != "2026-10-01" ||
		f.WaybillIssueDateTo.Format("2006-01-02") != "2026-10-31" {
		t.Errorf("waybill-даты = %v .. %v", f.WaybillIssueDateFrom, f.WaybillIssueDateTo)
	}
	if f.AssignedAtFrom == nil || f.AssignedAtTo == nil || f.UnassignedAtFrom == nil || f.UnassignedAtTo == nil {
		t.Fatal("границы дат assigned/unassigned = nil")
	}
}

func TestAssignmentHandler_List_InvalidParams(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"мусор в id", "/api/v1/assignments?equipment_id=abc"},
		{"нулевой id", "/api/v1/assignments?waybill_id=0"},
		{"мусорный target_type", "/api/v1/assignments?target_type=shelf"},
		{"мусорный active", "/api/v1/assignments?active=yes"},
		{"мусор в дату", "/api/v1/assignments?assigned_at_from=garbage"},
		{"from > to", "/api/v1/assignments?assigned_at_from=2026-10-31&assigned_at_to=2026-10-01"},
		{"issue_date from > to", "/api/v1/assignments?waybill_issue_date_from=2026-11-01&waybill_issue_date_to=2026-10-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAssignmentHandler(&stubAssignmentService{}, discardLogger())
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()

			h.List(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rr.Code)
			}
		})
	}
}
