package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewCard_Valid(t *testing.T) {
	card, err := NewCard(42, "Иванов Иван Иванович")
	if err != nil {
		t.Fatalf("NewCard() неожиданная ошибка: %v", err)
	}
	if card.Number != 42 {
		t.Errorf("Number = %d, want 42", card.Number)
	}
	if card.FullName != "Иванов Иван Иванович" {
		t.Errorf("FullName = %q", card.FullName)
	}
	if !card.IsActive {
		t.Error("новая карточка должна быть активной")
	}
	if card.DeletedAt != nil {
		t.Error("DeletedAt должен быть nil у новой карточки")
	}
	if card.CreatedAt.IsZero() || card.UpdatedAt.IsZero() {
		t.Error("CreatedAt/UpdatedAt должны быть заполнены")
	}
}

func TestNewCard_InvalidNumber(t *testing.T) {
	for _, num := range []int{0, -1, -100} {
		_, err := NewCard(num, "Петров П.П.")
		if !errors.Is(err, ErrInvalidNumber) {
			t.Errorf("NewCard(%d): err = %v, want ErrInvalidNumber", num, err)
		}
	}
}

func TestNewCard_InvalidName(t *testing.T) {
	cases := map[string]string{
		"пустое ФИО":      "",
		"только пробел":   "   ",
		"слишком длинное": strings.Repeat("я", maxFullNameLen+1),
	}
	for name, tc := range cases {
		_, err := NewCard(1, tc)
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("NewCard name=%q (%s): err = %v, want ErrInvalidName", tc, name, err)
		}
	}
}

func TestNewCard_MaxNameLength(t *testing.T) {
	// Имя ровно на границе лимита — валидно.
	card, err := NewCard(1, strings.Repeat("я", maxFullNameLen))
	if err != nil {
		t.Fatalf("NewCard с именем на границе лимита: неожиданная ошибка: %v", err)
	}
	if card == nil {
		t.Fatal("NewCard вернул nil без ошибки")
	}
}

func TestNewCard_NormalizesName(t *testing.T) {
	// Краевые и внутренние пробелы схлопываются в одинарные.
	card, err := NewCard(1, "  Иванов   Иван  ")
	if err != nil {
		t.Fatalf("NewCard() ошибка: %v", err)
	}
	if card.FullName != "Иванов Иван" {
		t.Errorf("FullName = %q, want %q", card.FullName, "Иванов Иван")
	}
}

func TestCard_UpdateFullName(t *testing.T) {
	card, _ := NewCard(1, "Старое Имя")
	before := card.UpdatedAt

	if err := card.UpdateFullName("Новое Имя"); err != nil {
		t.Fatalf("UpdateFullName() ошибка: %v", err)
	}
	if card.FullName != "Новое Имя" {
		t.Errorf("FullName = %q, want %q", card.FullName, "Новое Имя")
	}
	if !card.UpdatedAt.After(before) {
		t.Error("UpdatedAt должен обновиться")
	}
}

func TestCard_UpdateFullName_Invalid(t *testing.T) {
	card, _ := NewCard(1, "Действующее Имя")
	for _, bad := range []string{"", "   "} {
		if err := card.UpdateFullName(bad); !errors.Is(err, ErrInvalidName) {
			t.Errorf("UpdateFullName(%q): err = %v, want ErrInvalidName", bad, err)
		}
	}
	// Имя не должно измениться при ошибке валидации.
	if card.FullName != "Действующее Имя" {
		t.Errorf("FullName изменился после ошибки: %q", card.FullName)
	}
}

func TestCard_Deactivate(t *testing.T) {
	card, _ := NewCard(1, "Иван")
	card.Deactivate()

	if card.IsActive {
		t.Error("после Deactivate карточка не должна быть активной")
	}
	if card.DeletedAt == nil {
		t.Error("после Deactivate DeletedAt должен быть заполнен")
	}
}

func TestCard_Deactivate_Idempotent(t *testing.T) {
	card, _ := NewCard(1, "Иван")
	card.Deactivate()
	first := *card.DeletedAt
	updatedAt := card.UpdatedAt

	// Повторная деактивация не должна перезаписывать DeletedAt/UpdatedAt.
	card.Deactivate()
	if !card.DeletedAt.Equal(first) {
		t.Errorf("DeletedAt изменился при повторной деактивации: %v → %v", first, *card.DeletedAt)
	}
	if !card.UpdatedAt.Equal(updatedAt) {
		t.Error("UpdatedAt изменился при повторной деактивации")
	}
}

func TestCardFilter_ZeroValue(t *testing.T) {
	var f CardFilter
	if f.FullName != nil || f.CreatedAtFrom != nil || f.CreatedAtTo != nil ||
		f.UpdatedAtFrom != nil || f.UpdatedAtTo != nil ||
		f.DeletedAtFrom != nil || f.DeletedAtTo != nil {
		t.Errorf("нулевой фильтр должен иметь nil-поля: %+v", f)
	}
}
