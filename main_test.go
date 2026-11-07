package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"kanban/internal/store"
	"kanban/internal/ui"
)

// TestSaverPersistsInOrder cobre o gravador do main: cada SaveMsg vira disco
// e o desfecho volta pelo callback, em ordem.
func TestSaverPersistsInOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.json")
	if err := store.Save(path, store.Default()); err != nil {
		t.Fatal(err)
	}
	board, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan ui.SaveResultMsg, 4)
	s := newSaver(func(r ui.SaveResultMsg) { results <- r })

	for i, title := range []string{"um", "dois", "tres"} {
		board.Columns[0].Cards = append(board.Columns[0].Cards, title)
		s.save(ui.SaveMsg{Board: board, Path: path})
		if r := <-results; r.Err != nil {
			t.Fatalf("gravação %d falhou: %v", i, r.Err)
		}
	}
	s.stop()

	got, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Columns[0].Cards, ",") != "um,dois,tres" {
		t.Errorf("disco deveria ter as 3 gravações na ordem, obtive %#v", got.Columns[0].Cards)
	}
	if _, err := s.result(); err != nil {
		t.Errorf("result: %v", err)
	}
}

// TestSaverDetectsConflict verifica que gravar por cima do arquivo de outra
// instância gera board.json.bak com a versão que estava no disco.
func TestSaverDetectsConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.json")
	ours := store.Default()
	ours.Columns[0].Cards = []string{"nossa"}
	if err := store.Save(path, ours); err != nil {
		t.Fatal(err)
	}
	ours, err := store.Load(path) // ModTime da nossa leitura
	if err != nil {
		t.Fatal(err)
	}

	other := store.Default()
	other.Columns[0].Cards = []string{"da outra instância"}
	if err := store.Save(path, other); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil { // garante mtime diferente
		t.Fatal(err)
	}

	results := make(chan ui.SaveResultMsg, 1)
	s := newSaver(func(r ui.SaveResultMsg) { results <- r })
	s.save(ui.SaveMsg{Board: ours, Path: path})
	r := <-results
	s.stop()

	if !r.Conflicted {
		t.Error("Callback deveria reportar conflito (arquivo mudou desde o último save)")
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil || !strings.Contains(string(bak), "da outra instância") {
		t.Errorf(".bak deveria guardar a versão do disco: err=%v\n%s", err, bak)
	}
	if conf, _ := s.result(); !conf {
		t.Error("result() deveria lembrar do conflito")
	}
}

// TestSaveSinkWired garante o contrato main↔ui: o main registra um destino e
// todo SaveMsg emitido pelo Update chega nele (e continua vindo pelo Cmd).
func TestSaveSinkWired(t *testing.T) {
	got := make(chan ui.SaveMsg, 1)
	ui.SetSaveDst(func(m ui.SaveMsg) { got <- m })
	defer ui.SetSaveDst(nil)

	b := store.Default()
	b.Columns[0].Cards = []string{"a"}
	var cur tea.Model = ui.New(b, "/tmp/sink.json")
	var cmd tea.Cmd
	cur, cmd = cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}) // move "a" p/ Fazendo
	if cmd == nil {
		t.Fatal("mover deveria pedir save")
	}
	select {
	case m := <-got:
		if m.Path != "/tmp/sink.json" || len(m.Board.Columns[1].Cards) != 1 {
			t.Errorf("SaveMsg inesperado: %#v", m)
		}
	case <-time.After(time.Second):
		t.Error("SaveDst não recebeu o SaveMsg")
	}
	if _, ok := cmd().(ui.SaveMsg); !ok {
		t.Error("o Cmd também deveria devolver SaveMsg")
	}
}
