package importer

import "testing"

func TestMapStatus(t *testing.T) {
	tests := []struct {
		in   string
		want equipmentStatus
	}{
		{"эксп", statusExp},
		{"ЭКСП", statusExp},
		{"эксп(СП)", statusExpSP},
		{"эксп(сп)", statusExpSP},
		{"эксп(инт)", statusExpInt},
		{"эксп(инт)-", statusExpInt},
		{"эксп(инт)*", statusExpInt},
		{"неиспр", statusBroken},
		{"", statusExp},
		{"мусор", statusExp},
	}
	for _, tt := range tests {
		if got := mapStatus(tt.in); got != tt.want {
			t.Errorf("mapStatus(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestClassifyTarget(t *testing.T) {
	tests := []struct {
		in       string
		wantType targetType
		wantDept string
	}{
		{"СКЛАД", targetWarehouse, ""},
		{"склад", targetWarehouse, ""},
		{" склад ", targetWarehouse, ""},
		{"СКЛАД/ЗПУ", targetDepartment, "склад/зпу"},
		{"склад/Добруш", targetDepartment, "склад/добруш"},
		{"Гомель ОПК", targetDepartment, "опк гомель"},
		{"Новая гута ОПК", targetDepartment, "опк новая гута"},
		{"Лоев ПОГК", targetDepartment, "погк лоев"},
		{"Круговец-Калинино ПОГЗ", targetDepartment, "погз круговец-калинино"},
		{"Мышковец В.В.", targetEmployee, ""},
		{"!", targetEmployee, ""},
		{"ТСПК файловый сервер", targetEmployee, ""},
	}
	for _, tt := range tests {
		gotType, gotDept := classifyTarget(tt.in)
		if gotType != tt.wantType || gotDept != tt.wantDept {
			t.Errorf("classifyTarget(%q) = (%q, %q), want (%q, %q)",
				tt.in, gotType, gotDept, tt.wantType, tt.wantDept)
		}
	}
}

func TestParseCardRef(t *testing.T) {
	tests := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"51", 51, true},
		{"51.0", 51, true},
		{"3", 3, true},
		{"", 0, false},
		{"abc", 0, false},
		{"51.5", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseCardRef(tt.in)
		if ok != tt.wantOK {
			t.Errorf("parseCardRef(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			continue
		}
		if ok && *got != tt.want {
			t.Errorf("parseCardRef(%q) = %d, want %d", tt.in, *got, tt.want)
		}
	}
}

func TestParseYearMonth(t *testing.T) {
	tests := []struct {
		in       string
		wantYear int
		wantMon  int
		wantOK   bool
	}{
		{"03.2015", 2015, 3, true},
		{"07/2007", 2007, 7, true},
		{"2015", 2015, 1, true},
		{"2010", 2010, 1, true},
		{"08.2012(03.2015)", 2015, 3, true}, // берём год в скобках
		{"", 0, 0, false},
		{"мусор", 0, 0, false},
		{"13.2015", 2015, 1, true}, // некорректный месяц → 1
	}
	for _, tt := range tests {
		y, m, ok := parseYearMonth(tt.in)
		if ok != tt.wantOK || y != tt.wantYear || m != tt.wantMon {
			t.Errorf("parseYearMonth(%q) = (%d, %d, %v), want (%d, %d, %v)",
				tt.in, y, m, ok, tt.wantYear, tt.wantMon, tt.wantOK)
		}
	}
}
