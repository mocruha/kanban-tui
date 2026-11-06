package store

import "testing"

func TestDefault(t *testing.T) {
	b := Default()
	if len(b.Columns) != 3 {
		t.Fatalf("queria 3 colunas, obtive %d", len(b.Columns))
	}
	want := []string{"A Fazer", "Fazendo", "Feito"}
	for i, w := range want {
		if b.Columns[i].Title != w {
			t.Errorf("coluna %d: queria %q, obtive %q", i, w, b.Columns[i].Title)
		}
		if b.Columns[i].Cards == nil {
			t.Errorf("coluna %d: Cards deve ser fatia vazia não-nil", i)
		}
	}
}