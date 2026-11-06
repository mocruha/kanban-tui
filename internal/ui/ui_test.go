package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"kanban/internal/store"
)

func k(s string) tea.Msg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send aplica uma sequência de teclas e devolve o modelo resultante.
func send(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	var cur tea.Model = m
	for _, key := range keys {
		cur, _ = cur.Update(k(key))
	}
	out, ok := cur.(Model)
	if !ok {
		t.Fatal("Update deveria devolver ui.Model por valor")
	}
	return out
}

func newTestModel() Model {
	return New(store.Default(), "/tmp/board-test.json")
}

func TestNavigationArrows(t *testing.T) {
	m := newTestModel()
	m = send(t, m, "right")
	if m.col != 1 {
		t.Errorf("right: queria col 1, obtive %d", m.col)
	}
	m = send(t, m, "left", "left", "left") // clamp na esquerda
	if m.col != 0 {
		t.Errorf("clamp: queria col 0, obtive %d", m.col)
	}
	m = send(t, m, "right", "right", "right") // clamp na direita
	if m.col != 2 {
		t.Errorf("clamp: queria col 2, obtive %d", m.col)
	}
	m = send(t, m, "up", "down") // colunas vazias: card fica 0, sem pânico
	if m.card != 0 {
		t.Errorf("card em coluna vazia deveria ser 0, obtive %d", m.card)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := newTestModel()
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	out := cur.(Model)
	if out.visibleRows != 6 { // 12 - 6 (margem fixa), dentro de [3,50]
		t.Errorf("visibleRows: queria 6, obtive %d", out.visibleRows)
	}
}

func seededModel(t *testing.T, col0 ...string) Model {
	t.Helper()
	b := store.Default()
	b.Columns[0].Cards = append(b.Columns[0].Cards, col0...)
	return New(b, "/tmp/board-test.json")
}

func TestMoveCardToColumn(t *testing.T) {
	m := seededModel(t, "a", "b")
	m = send(t, m, "down") // foca "b"
	m = send(t, m, "2")    // move para Fazendo
	if got := m.cards(0); len(got) != 1 || got[0] != "a" {
		t.Errorf("A Fazer deveria sobrar [a], obtive %#v", got)
	}
	if got := m.cards(1); len(got) != 1 || got[0] != "b" {
		t.Errorf("Fazendo deveria ter [b], obtive %#v", got)
	}
	if m.col != 1 || m.card != 0 {
		t.Errorf("cursor deveria seguir o card p/ (col 1, card 0), obtive (%d,%d)", m.col, m.card)
	}
	m = send(t, m, "2") // reclique o dígito da coluna atual: no-op, não duplica
	if got := m.cards(1); len(got) != 1 {
		t.Errorf("mover p/ a mesma coluna não deveria duplicar: %#v", got)
	}
}

func TestAddCardByTyping(t *testing.T) {
	m := send(t, newTestModel(), "n", "o", "i", "enter") // n abre input; digita "oi"
	if got := m.cards(0); len(got) != 1 || got[0] != "oi" {
		t.Errorf("novo card: queria [oi], obtive %#v", got)
	}
	if m.mode != modeBrowse {
		t.Errorf("Enter deveria voltar a browse, modo=%d", m.mode)
	}
	if m.col != 0 || m.card != 0 {
		t.Errorf("cursor deveria apontar p/ o card novo (0,0), obtive (%d,%d)", m.col, m.card)
	}
}

func TestAddEmptyTitleDiscarded(t *testing.T) {
	m := send(t, newTestModel(), "n", " ", "enter") // só espaço -> trim -> descarta
	if got := m.cards(0); len(got) != 0 {
		t.Errorf("título vazio não deveria criar card: %#v", got)
	}
}

func TestAddCancelledByEsc(t *testing.T) {
	m := send(t, newTestModel(), "n", "a", "b", "esc")
	if got := m.cards(0); len(got) != 0 {
		t.Errorf("Esc deveria cancelar sem gravar: %#v", got)
	}
	if m.mode != modeBrowse {
		t.Error("Esc no editor deveria voltar a browse")
	}
}

func TestEnterOpensEditorWithPrefilledValue(t *testing.T) {
	m := seededModel(t, "comprar pão")
	m = send(t, m, "enter")
	if m.mode != modeEdit {
		t.Fatalf("Enter sobre card deveria abrir modoEdit")
	}
	if m.ti.Value() != "comprar pão" {
		t.Errorf("input pré-carregado: queria %q, obtive %q", "comprar pão", m.ti.Value())
	}
	if m.editingIdx != 0 {
		t.Errorf("editingIdx deveria ser 0, obtive %d", m.editingIdx)
	}
}

func TestEditCancelledKeepsOriginal(t *testing.T) {
	m := seededModel(t, "antigo")
	m = send(t, m, "enter", "x", "y", "esc") // digita e cancela
	if got := m.cards(0); len(got) != 1 || got[0] != "antigo" {
		t.Errorf("Esc na edição deveria preservar o título: %#v", got)
	}
}

func TestCommitEditReplacesInPlace(t *testing.T) {
	m := seededModel(t, "um", "dois", "tres")
	m = send(t, m, "down", "down", "enter") // abre edição de "tres"
	m.ti.SetValue("quatro")
	m = send(t, m, "enter") // confirma
	if got := m.cards(0); len(got) != 3 || got[2] != "quatro" {
		t.Errorf("edição in-place: queria [um dois quatro], obtive %#v", got)
	}
	if m.card != 2 {
		t.Errorf("cursor deveria ficar no card editado (2), obtive %d", m.card)
	}
}

func TestCommitNewAppendsAtEnd(t *testing.T) {
	m := seededModel(t, "um")
	m = send(t, m, "n")
	m.ti.SetValue("dois")
	m = send(t, m, "enter")
	if got := m.cards(0); len(got) != 2 || got[1] != "dois" || m.card != 1 {
		t.Errorf("novo card vai p/ o fim com cursor: %#v cursor=(%d,%d)", got, m.col, m.card)
	}
}

func TestDeleteCard(t *testing.T) {
	m := seededModel(t, "a", "b", "c")
	m = send(t, m, "down", "down") // foca "c"
	m = send(t, m, "d")
	if got := m.cards(0); len(got) != 2 || got[1] != "b" {
		t.Errorf("apagar 'c': queria [a b], obtive %#v", got)
	}
	if m.card != 1 {
		t.Errorf("cursor deveria ter feito clamp p/ 1, obtive %d", m.card)
	}
	if m.undo == nil || m.undo.title != "c" || m.undo.idx != 2 {
		t.Errorf("undo deveria guardar {col 0, idx 2, c}, obtive %#v", m.undo)
	}
}

func TestUndoRestoresInPlace(t *testing.T) {
	m := seededModel(t, "a", "b", "c")
	m = send(t, m, "down", "d") // apaga "b"
	m = send(t, m, "u")
	if got := m.cards(0); len(got) != 3 || got[1] != "b" {
		t.Errorf("undo deveria reinserir 'b' no índice 1: %#v", got)
	}
	if m.col != 0 || m.card != 1 {
		t.Errorf("cursor deveria ir p/ o card restaurado, obtive (%d,%d)", m.col, m.card)
	}
	m = send(t, m, "u") // segundo u: no-op
	if len(m.cards(0)) != 3 {
		t.Error("undo sem estoque deveria ser no-op")
	}
}

func TestMoveClearsUndo(t *testing.T) {
	m := seededModel(t, "a", "b")
	m = send(t, m, "d")
	if m.undo == nil {
		t.Fatal("d deveria gravar undo")
	}
	m = send(t, m, "2")
	if m.undo != nil {
		t.Error("mover deveria limpar undo")
	}
}

func TestDeleteOnEmptyColumnSafe(t *testing.T) {
	m := send(t, newTestModel(), "d") // nenhuma coluna tem card
	if m.undo != nil {
		t.Error("apagar em coluna vazia deveria ser no-op")
	}
}