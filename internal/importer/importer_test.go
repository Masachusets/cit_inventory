package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name   string
		want   fileVersion
		wantOK bool
	}{
		{name: "Номенклатурная таблица 01.2026.xlsx", want: fileVersion{Month: 1, Year: 2026}, wantOK: true},
		{name: "Номенклатурная таблица 12.2025.xlsx", want: fileVersion{Month: 12, Year: 2025}, wantOK: true},
		{name: "Номенклатурная таблица 1.2026.xlsx", want: fileVersion{Month: 1, Year: 2026}, wantOK: true},
		{name: "UMS.xlsx", wantOK: false},
		{name: "Номенклатурная таблица 13.2026.xlsx", wantOK: false},
		{name: "Номенклатурная таблица 01.2026.xlsm", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseVersion(tt.name)
			if ok != tt.wantOK {
				t.Fatalf("parseVersion(%q) ok = %v, want %v", tt.name, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("parseVersion(%q) = %+v, want %+v", tt.name, got, tt.want)
			}
		})
	}
}

func TestNewer(t *testing.T) {
	tests := []struct {
		a, b fileVersion
		want bool
	}{
		{fileVersion{Month: 2, Year: 2026}, fileVersion{Month: 1, Year: 2026}, true},
		{fileVersion{Month: 1, Year: 2026}, fileVersion{Month: 12, Year: 2025}, true},
		{fileVersion{Month: 12, Year: 2025}, fileVersion{Month: 1, Year: 2026}, false},
		{fileVersion{Month: 1, Year: 2026}, fileVersion{Month: 1, Year: 2026}, false},
	}
	for _, tt := range tests {
		if got := newer(tt.a, tt.b); got != tt.want {
			t.Errorf("newer(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestResolveFile(t *testing.T) {
	t.Run("указан файл напрямую", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "Номенклатурная таблица 03.2026.xlsx")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, ver, err := resolveFile(path)
		if err != nil {
			t.Fatalf("resolveFile(): %v", err)
		}
		if got != path {
			t.Errorf("path = %q, want %q", got, path)
		}
		if want := (fileVersion{Month: 3, Year: 2026}); ver != want {
			t.Errorf("version = %+v, want %+v", ver, want)
		}
	})

	t.Run("директория: выбирается последняя версия", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{
			"Номенклатурная таблица 01.2026.xlsx",
			"Номенклатурная таблица 12.2025.xlsx",
			"Номенклатурная таблица 11.2025.xlsx",
			"UMS.xlsx", // не номенклатурная — игнорируется
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		got, ver, err := resolveFile(dir)
		if err != nil {
			t.Fatalf("resolveFile(): %v", err)
		}
		want := filepath.Join(dir, "Номенклатурная таблица 01.2026.xlsx")
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if want := (fileVersion{Month: 1, Year: 2026}); ver != want {
			t.Errorf("version = %+v, want %+v", ver, want)
		}
	})

	t.Run("директория без номенклатурных файлов — ошибка", func(t *testing.T) {
		if _, _, err := resolveFile(t.TempDir()); err == nil {
			t.Error("resolveFile() error = nil, want error")
		}
	})

	t.Run("несуществующий путь — ошибка", func(t *testing.T) {
		if _, _, err := resolveFile(filepath.Join(t.TempDir(), "нет")); err == nil {
			t.Error("resolveFile() error = nil, want error")
		}
	})
}

func TestResolveUMSFile(t *testing.T) {
	t.Run("указан файл напрямую", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "UMS.xlsx")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := resolveUMSFile(path)
		if err != nil {
			t.Fatalf("resolveUMSFile(): %v", err)
		}
		if got != path {
			t.Errorf("path = %q, want %q", got, path)
		}
	})

	t.Run("директория: ищется UMS.xlsx", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "UMS.xlsx"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := resolveUMSFile(dir)
		if err != nil {
			t.Fatalf("resolveUMSFile(): %v", err)
		}
		want := filepath.Join(dir, "UMS.xlsx")
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})

	t.Run("директория без UMS.xlsx — ошибка", func(t *testing.T) {
		if _, err := resolveUMSFile(t.TempDir()); err == nil {
			t.Error("resolveUMSFile() error = nil, want error")
		}
	})

	t.Run("несуществующий путь — ошибка", func(t *testing.T) {
		if _, err := resolveUMSFile(filepath.Join(t.TempDir(), "нет")); err == nil {
			t.Error("resolveUMSFile() error = nil, want error")
		}
	})
}
