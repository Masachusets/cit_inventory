package web

import (
	"context"
	"strings"
	"testing"
)

// TestLayout_Render проверяет, что базовый layout содержит каркас страницы:
// заголовок, ссылки на CSS/htmx и пункты навигации.
func TestLayout_Render(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		nav     []NavItem
		want    []string
		notWant []string
	}{
		{
			name:  "базовый layout",
			title: "Оборудование",
			nav:   DefaultNav(),
			want: []string{
				"<!doctype html>",
				`<title>Оборудование · CIT Inventory</title>`,
				`lang="ru"`,
				`href="/static/app.css"`,
				`src="/static/htmx.min.js"`,
				`href="/ui/equipments"`,
				"Оборудование",
				`<main id="content">`,
			},
		},
		{
			name:  "пустая навигация",
			title: "Главная",
			nav:   nil,
			want: []string{
				`<title>Главная · CIT Inventory</title>`,
				`src="/static/htmx.min.js"`,
			},
			notWant: []string{`href="/ui/equipments"`},
		},
		{
			name:  "спецсимволы в названии экранируются",
			title: "Поиск <script>",
			nav:   []NavItem{},
			want: []string{
				"<title>Поиск &lt;script&gt; · CIT Inventory</title>",
			},
			notWant: []string{"<title>Поиск <script>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			err := Layout(tt.title, tt.nav).Render(context.Background(), &sb)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			got := sb.String()

			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("ожидалось в разметке: %q", w)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(got, nw) {
					t.Errorf("не ожидалось в разметке: %q", nw)
				}
			}
		})
	}
}

// TestStaticFS проверяет, что статические ресурсы встроены в бинарник
// и содержимое htmx/CSS доступно через embed.FS.
func TestStaticFS(t *testing.T) {
	for _, name := range []string{"static/htmx.min.js", "static/app.css"} {
		data, err := Static.ReadFile(name)
		if err != nil {
			t.Fatalf("чтение %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Errorf("%s пуст", name)
		}
	}

	js, err := Static.ReadFile("static/htmx.min.js")
	if err != nil {
		t.Fatalf("чтение static/htmx.min.js: %v", err)
	}
	if !strings.Contains(string(js), "htmx") {
		t.Error("static/htmx.min.js не похож на htmx")
	}
}
