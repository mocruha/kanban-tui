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

func TestLoadMissingFile(t *testing.T) {
	b, err := Load("/caminho/que/nao/existe/board.json")
	if err != nil {
		t.Fatalf("arquivo inexistente deve dar board default, não erro: %v", err)
	}
	if len(b.Columns) != 3 {
		t.Fatalf("default tinha que ter 3 colunas")
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/board.json"
	content := `{"columns":[
		{"title":"A Fazer","cards":["escrever planilha"]},
		{"title":"Fazendo","cards":[]},
		{"title":"Feito","cards":["café","boleca"]}]}`
	if err := writeFile(p, content); err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Columns[2].Cards) != 2 || b.Columns[2].Cards[0] != "café" {
		t.Errorf("coluna Feito incorreta: %#v", b.Columns[2].Cards)
	}
	if b.Columns[1].Cards == nil {
		t.Errorf("cards omitidos/null devem virar fatia vazia não-nil")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/board.json"
	if err := writeFile(p, "isso não é json {{{"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("JSON corrompido deve retornar erro, não board")
	}
}

func TestLoadWrongColumnCount(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/board.json"
	if err := writeFile(p, `{"columns":[{"title":"A Fazer","cards":[]}]}`); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("1 coluna deve dar erro (esperava 3)")
	}
}