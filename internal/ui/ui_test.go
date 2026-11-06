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