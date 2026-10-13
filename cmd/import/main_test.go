package main

import (
	"os"
	"strings"
	"testing"
)

// withArgs подменяет os.Args на время теста и восстанавливает исходное значение.
func withArgs(t *testing.T, args ...string) {
	t.Helper()

	orig := os.Args
	os.Args = append([]string{"import"}, args...)
	t.Cleanup(func() { os.Args = orig })
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{
			name: "нет аргументов — импорт из Excel",
			args: nil,
			want: cmdImport,
		},
		{
			name: "первый аргумент — флаг, импорт из Excel",
			args: []string{"-excel=data/excel", "-ums="},
			want: cmdImport,
		},
		{
			name: "значение флага через пробел не считается подкомандой",
			args: []string{"-excel", "data/excel"},
			want: cmdImport,
		},
		{
			name: "явная подкоманда import",
			args: []string{cmdImport, "-ums=data/excel/UMS.xlsx"},
			want: cmdImport,
		},
		{
			name: "подкоманда departments",
			args: []string{cmdDepartments, "-file=data/departments.txt"},
			want: cmdDepartments,
		},
		{
			name: "подкоманда departments без флагов",
			args: []string{cmdDepartments},
			want: cmdDepartments,
		},
		{
			name: "подкоманда help",
			args: []string{cmdHelp},
			want: cmdHelp,
		},
		{
			name:    "неизвестная подкоманда",
			args:    []string{"cards"},
			wantErr: "неизвестная подкоманда",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withArgs(t, tt.args...)

			got, err := parseCommand()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseCommand() error = nil, want it to contain %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCommand(): %v", err)
			}
			if got != tt.want {
				t.Errorf("cmd = %q, want %q", got, tt.want)
			}

			// Подкоманда должна быть удалена из os.Args, чтобы flag.Parse()
			// внутри config.Load() не споткнулся о неё.
			for _, arg := range os.Args[1:] {
				if arg == tt.want {
					t.Errorf("os.Args = %q, подкоманда %q не удалена", os.Args, tt.want)
				}
			}
		})
	}
}

// TestParseCommand_StripsSubcommand проверяет, что из os.Args удаляется именно
// подкоманда, а флаги после неё сохраняются в исходном порядке.
func TestParseCommand_StripsSubcommand(t *testing.T) {
	withArgs(t, cmdDepartments, "-file=data/departments.txt", "-debug")

	got, err := parseCommand()
	if err != nil {
		t.Fatalf("parseCommand(): %v", err)
	}
	if got != cmdDepartments {
		t.Fatalf("cmd = %q, want %q", got, cmdDepartments)
	}

	want := []string{"-file=data/departments.txt", "-debug"}
	if len(os.Args[1:]) != len(want) {
		t.Fatalf("os.Args[1:] = %q, want %q", os.Args[1:], want)
	}
	for i, arg := range want {
		if os.Args[1:][i] != arg {
			t.Fatalf("os.Args[1:] = %q, want %q", os.Args[1:], want)
		}
	}
}

// TestUsage перечисляет все поддерживаемые подкоманды.
func TestUsage(t *testing.T) {
	for _, cmd := range []string{cmdDepartments, cmdHelp} {
		if !strings.Contains(usage, cmd) {
			t.Errorf("usage не упоминает подкоманду %q", cmd)
		}
	}
}
