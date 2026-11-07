package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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

func TestLoadRejectsZeroColumns(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/board.json"
	if err := writeFile(p, `{"columns":[]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("0 colunas deve dar erro")
	}
}

func TestLoadAcceptsVariableColumns(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/board.json"
	content := `{"columns":[
		{"title":"Backlog","cards":[]},
		{"title":"A Fazer","cards":["x"]},
		{"title":"Fazendo","cards":[]},
		{"title":"Feito","cards":[]}]}`
	if err := writeFile(p, content); err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	if err != nil {
		t.Fatalf("4 colunas deveriam carregar: %v", err)
	}
	if len(b.Columns) != 4 || b.Columns[0].Title != "Backlog" || b.Columns[1].Cards[0] != "x" {
		t.Errorf("colunas customizadas deveriam ser preservadas: %#v", b.Columns)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	p := t.TempDir() + "/sub/board.json"
	b := Default()
	b.Columns[0].Cards = append(b.Columns[0].Cards, "café ☕", "ir ao médico")
	if err := Save(p, b); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("Save deveria criar diretório e arquivo: %v", err)
	}
	if !strings.Contains(string(data), "\"cards\"") {
		t.Errorf("JSON deveria ser indentado legível, obtive:\n%s", data)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("arquivo temporário deveria ter sumido, dir tem %d entradas", len(entries))
	}
	b2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	b.ModTime = b2.ModTime // mtime é estado de leitura, não conteúdo
	if !reflect.DeepEqual(b, b2) {
		t.Errorf("round-trip divergiu:\n%#v\n%#v", b, b2)
	}
}

func TestLoadKeepsTitlesAndModTime(t *testing.T) {
	p := t.TempDir() + "/board.json"
	b := Default()
	b.Columns[0].Title = "Backlog" // colunas renomeadas à mão são preservadas
	if err := Save(p, b); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Columns[0].Title != "Backlog" {
		t.Errorf("título customizado deveria ser preservado, obtive %q", got.Columns[0].Title)
	}
	if got.ModTime.IsZero() {
		t.Error("ModTime deveria ser preenchido pelo Load")
	}
}

func TestLoadRejectsEmptyTitle(t *testing.T) {
	p := t.TempDir() + "/board.json"
	if err := writeFile(p, `{"columns":[{"title":"","cards":[]},{"title":"B","cards":[]},{"title":"C","cards":[]}]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("coluna sem título deve dar erro")
	}
}

func TestLoadRejectsDuplicateTitles(t *testing.T) {
	p := t.TempDir() + "/board.json"
	if err := writeFile(p, `{"columns":[{"title":"X","cards":[]},{"title":"X","cards":[]},{"title":"C","cards":[]}]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("títulos repetidos devem dar erro")
	}
}

func TestLoadRejectsOversizedFile(t *testing.T) {
	p := t.TempDir() + "/board.json"
	big := make([]byte, MaxFileSize+1)
	if err := os.WriteFile(p, big, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("arquivo acima do limite deve dar erro claro, não travar")
	}
}

func TestSaveIfUnchangedNoConflict(t *testing.T) {
	p := t.TempDir() + "/board.json"
	b := Default()
	b.Columns[0].Cards = []string{"a"}
	if err := Save(p, b); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Columns[0].Cards = append(loaded.Columns[0].Cards, "b")
	conflicted, err := SaveIfUnchanged(p, loaded)
	if err != nil || conflicted {
		t.Fatalf("sem conflito: conflicted=%v err=%v", conflicted, err)
	}
	if _, err := os.Stat(p + ".bak"); !os.IsNotExist(err) {
		t.Error("não deveria criar .bak sem conflito")
	}
}

func TestSaveIfUnchangedBacksUpConflict(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/board.json"
	b := Default()
	if err := Save(p, b); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	// Outra instância (ou um editor) grava por cima depois da leitura.
	other := Default()
	other.Columns[1].Cards = []string{"da outra instância"}
	if err := Save(p, other); err != nil {
		t.Fatal(err)
	}
	conflicted, err := SaveIfUnchanged(p, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if !conflicted {
		t.Fatal("mtime mudou desde o Load: deveria detectar conflito")
	}
	bak, err := os.ReadFile(p + ".bak")
	if err != nil || !strings.Contains(string(bak), "da outra instância") {
		t.Errorf(".bak deveria guardar a versão do disco: err=%v\n%s", err, bak)
	}
	now, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(now.Columns[1].Cards) != 0 {
		t.Error("o arquivo principal deveria conter a versão salva pela nossa instância")
	}
}
