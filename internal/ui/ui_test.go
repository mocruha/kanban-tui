package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"kanban/internal/store"
)

// shift transforma a letra em maiúscula como o terminal entrega Shift+letra.
func shift(s string) string { return strings.ToUpper(s) }

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
	case "J", "K":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Alt: false}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send aplica uma sequência de teclas e devolve o modelo resultante.
func send(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	var cur tea.Model = m
	for _, key := range keys {
		if len(key) == 1 && key[0] >= 'A' && key[0] <= 'Z' {
			key = shift(key)
		}
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
	if m.mode != modeConfirmDelete {
		t.Fatal("'d' deveria pedir confirmação")
	}
	if got := m.cards(0); len(got) != 3 {
		t.Fatalf("nada deveria ser apagado antes da confirmação: %#v", got)
	}
	m = send(t, m, "d") // confirma
	if got := m.cards(0); len(got) != 2 || got[1] != "b" {
		t.Errorf("apagar 'c': queria [a b], obtive %#v", got)
	}
	if m.card != 1 {
		t.Errorf("cursor deveria ter feito clamp p/ 1, obtive %d", m.card)
	}
	if len(m.undo) != 1 || m.undo[0].cards[0][2] != "c" {
		t.Errorf("undo deveria guardar o estado com 'c', obtive %#v", m.undo)
	}
}

func TestDeleteCancelledByOtherKey(t *testing.T) {
	m := seededModel(t, "a")
	m = send(t, m, "d", "q") // q cancela a confirmação, não sai
	if got := m.cards(0); len(got) != 1 {
		t.Errorf("outra tecla deveria cancelar o apagar: %#v", got)
	}
	if m.mode != modeBrowse {
		t.Errorf("deveria voltar a browse, modo=%d", m.mode)
	}
}

func TestDeleteAsksConfirmationInFooter(t *testing.T) {
	m := seededModel(t, "a")
	view := send(t, m, "d").View()
	if !strings.Contains(view, "Apagar") || !strings.Contains(view, "d confirma") {
		t.Errorf("footer deveria pedir confirmação:\n%s", view)
	}
}

func TestUndoRestoresInPlace(t *testing.T) {
	m := seededModel(t, "a", "b", "c")
	m = send(t, m, "down", "d", "d") // apaga "b"
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

func TestMoveIsUndoable(t *testing.T) {
	m := seededModel(t, "a", "b")
	m = send(t, m, "2") // move "a" para Fazendo
	if got := m.cards(1); len(got) != 1 || got[0] != "a" {
		t.Fatalf("mover falhou: %#v", got)
	}
	m = send(t, m, "u")
	if got := m.cards(0); len(got) != 2 || got[0] != "a" {
		t.Errorf("undo do mover deveria devolver 'a' ao topo: %#v", got)
	}
	if got := m.cards(1); len(got) != 0 {
		t.Errorf("Fazendo deveria voltar vazia: %#v", got)
	}
}

func TestUndoStackLevels(t *testing.T) {
	m := seededModel(t, "a")
	m = send(t, m, "n")
	m.ti.SetValue("b")
	m = send(t, m, "enter") // cria "b" (segue o cursor)
	m = send(t, m, "n")
	m.ti.SetValue("c")
	m = send(t, m, "enter")  // cria "c"
	m = send(t, m, "d", "d") // apaga "c"
	m = send(t, m, "2")      // move "b" p/ Fazendo
	if got := m.cards(0); len(got) != 1 || got[0] != "a" || len(m.cards(1)) != 1 {
		t.Fatalf("estado inesperado: %#v / %#v", got, m.cards(1))
	}
	m = send(t, m, "u", "u") // desfaz o move e o apagar
	if got := m.cards(0); len(got) != 3 || got[1] != "b" || got[2] != "c" || len(m.cards(1)) != 0 {
		t.Errorf("2 undos deveriam restaurar [a b c] e esvaziar Fazendo: %#v / %#v", got, m.cards(1))
	}
	m = send(t, m, "u") // desfaz a criação de "c"
	if got := m.cards(0); len(got) != 2 || got[1] != "b" {
		t.Errorf("3º undo deveria tirar 'c': %#v", got)
	}
	m = send(t, m, "ctrl+y") // refaz a criação de "c"
	if got := m.cards(0); len(got) != 3 || got[2] != "c" {
		t.Errorf("redo deveria recriar 'c': %#v", got)
	}
}

func TestUndoThenNewActionClearsRedo(t *testing.T) {
	m := seededModel(t, "a")
	m = send(t, m, "d", "d", "u")
	if len(m.redo) != 1 {
		t.Fatalf("redo deveria ter 1 entrada")
	}
	m = send(t, m, "n")
	m.ti.SetValue("b")
	m = send(t, m, "enter")
	if len(m.redo) != 0 {
		t.Error("nova alteração deveria limpar o redo")
	}
}

func TestFailedSaveReportedInFooter(t *testing.T) {
	m := seededModel(t, "a")
	cur, _ := m.Update(k("d")) // d: pede confirmação
	m = cur.(Model)
	m.dirty = true
	cur, _ = m.Update(SaveResultMsg{Err: errors.New("disco cheio")})
	view := cur.(Model).View()
	if !strings.Contains(view, "FALHOU SALVAR") || !strings.Contains(view, "disco cheio") {
		t.Errorf("footer deveria mostrar o erro de gravação:\n%s", view)
	}
}

func TestDeleteOnEmptyColumnSafe(t *testing.T) {
	m := send(t, newTestModel(), "d") // nenhuma coluna tem card
	if m.mode != modeBrowse || len(m.undo) != 0 {
		t.Error("apagar em coluna vazia deveria ser no-op")
	}
}

func TestViewContents(t *testing.T) {
	m := seededModel(t, "muito longo este título que não cabe na coluna e deve ser cortado")
	m = send(t, m, "n")        // modo editor para checar render do input
	m = send(t, m, "esc")      // volta a browse
	m.ti.SetValue("digitando") // (render do input é coberto em TestViewEditorVisible)
	view := m.View()
	for _, want := range []string{"A Fazer", "Fazendo", "Feito", "▸", "mover", "…"} {
		if !strings.Contains(view, want) {
			t.Errorf("View deveria conter %q. View:\n%s", want, view)
		}
	}
	if strings.Contains(view, "u desfazer") {
		t.Errorf("footer não deveria listar 'u' sem undo disponível:\n%s", view)
	}
}

func TestViewEditorVisible(t *testing.T) {
	m := newTestModel()
	m = send(t, m, "n")
	m.ti.SetValue("oi")
	view := m.View()
	if !strings.Contains(view, "oi") || !strings.Contains(view, "Enter confirma") {
		t.Errorf("modo editor deveria mostrar input + dica:\n%s", view)
	}
}

func TestViewRuneSafeTruncation(t *testing.T) {
	// "café" acentuado: truncamento por len() de bytes quebraria o "…".
	b := store.Default()
	b.Columns[0].Cards = []string{"café ☕ café ☕ café ☕ café ☕ café ☕ café ☕ café ☕ café ☕"}
	m := New(b, "/tmp/x.json")
	m.width = 42 // colunas estreitas -> forçar truncamento
	view := m.View()
	if !strings.Contains(view, "café") || !strings.Contains(view, "…") {
		t.Errorf("truncamento deveria preservar runes e indicar corte:\n%s", view)
	}
	// largura máxima: cada coluna (42-6)/3 = 12 outer -> 3 colunas não passam de 42+2
	for _, line := range strings.Split(view, "\n") {
		if utf8.RuneCountInString(line) > 44 {
			t.Errorf("linha mais larga que o terminal (%d runes): %q", utf8.RuneCountInString(line), line)
		}
	}
}

func TestTruncateCellWidth(t *testing.T) {
	// runes largas (CJK, emoji) ocupam 2 células: cortar por runes estoura a borda.
	for _, s := range []string{"日本語日本語日本語", "🎉🎉🎉🎉🎉🎉🎉🎉", "e\u0301e\u0301e\u0301e\u0301e\u0301"} {
		for w := 1; w <= 8; w++ {
			if got := runewidth.StringWidth(truncate(s, w)); got > w {
				t.Errorf("truncate(%q, %d) = %q tem %d células", s, w, truncate(s, w), got)
			}
		}
	}
}

func TestViewWideCharsFitColumns(t *testing.T) {
	b := store.Default()
	b.Columns[0].Cards = []string{"日本語のとても長いタイトル", "🎉 festa 🎉 festa 🎉"}
	b.Columns[1].Cards = []string{"日本語"}
	m := New(b, "/tmp/x.json")
	m.width = 42
	for _, line := range strings.Split(m.View(), "\n") {
		if w := runewidth.StringWidth(line); w > 44 {
			t.Errorf("linha com %d células (limite 44): %q", w, line)
		}
	}
}

func TestScrollKeepsFocusVisible(t *testing.T) {
	var cards []string
	for i := range 40 {
		cards = append(cards, fmt.Sprintf("card %02d", i))
	}
	m := seededModel(t, cards...)
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12}) // 6 linhas -> 5 visíveis
	m = cur.(Model)
	for range 39 {
		m = send(t, m, "down")
	}
	if m.card != 39 {
		t.Fatalf("cursor deveria estar no último card, obtive %d", m.card)
	}
	view := m.View()
	if !strings.Contains(view, "card 39") {
		t.Errorf("card focado deveria estar visível depois de rolar:\n%s", view)
	}
	if strings.Contains(view, "card 00") {
		t.Errorf("topo da lista deveria ter saído da janela:\n%s", view)
	}
	for range 39 {
		m = send(t, m, "up")
	}
	if !strings.Contains(m.View(), "card 00") {
		t.Errorf("voltar ao topo deveria mostrar o card 00")
	}
}

func TestReorderWithinColumn(t *testing.T) {
	m := seededModel(t, "a", "b", "c")
	m = send(t, m, "down", "J") // "b" desce
	if got := m.cards(0); got[0] != "a" || got[1] != "c" || got[2] != "b" {
		t.Errorf("J deveria descer 'b': %#v", got)
	}
	if m.card != 2 {
		t.Errorf("cursor deveria acompanhar o card (2), obtive %d", m.card)
	}
	m = send(t, m, "K", "K", "K") // sobe até o topo e para
	if got := m.cards(0); got[0] != "b" || got[1] != "a" || got[2] != "c" {
		t.Errorf("K deveria subir 'b' ao topo (e parar lá): %#v", got)
	}
	if m.card != 0 || len(m.undo) != 3 {
		t.Errorf("cursor=%d undos=%d (o K no topo não altera)", m.card, len(m.undo))
	}
}

func TestMoveToColumnThenReorderKeepsRelativeOrder(t *testing.T) {
	b := store.Default()
	b.Columns[0].Cards = []string{"a", "b"}
	b.Columns[1].Cards = []string{"x", "y", "z"}
	m := New(b, "/tmp/x.json")
	m = send(t, m, "right", "down", "down") // "z" em Fazendo
	m = send(t, m, "1")
	if got := m.cards(0); len(got) != 3 || got[2] != "z" {
		t.Errorf("deveria anexar 'z' no fim de A Fazer: %#v", got)
	}
	if m.col != 0 || m.card != 2 {
		t.Errorf("cursor deveria seguir p/ (0,2), obtive (%d,%d)", m.col, m.card)
	}
	if got := m.cards(1); len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("coluna de origem deveria manter ordem: %#v", got)
	}
}

func TestNoOpActionsDoNotSave(t *testing.T) {
	m := seededModel(t, "a")
	for _, key := range []string{"1", "up", "K", "left"} {
		if _, cmd := m.Update(k(key)); cmd != nil {
			t.Errorf("%q não altera nada, não deveria pedir save", key)
		}
	}
	if _, cmd := m.Update(k("2")); cmd == nil {
		t.Error("mover deveria pedir save")
	} else if save, ok := cmd().(SaveMsg); !ok || save.Path != m.Path || save.Board != m.Board {
		t.Errorf("comando de save deveria emitir ui.SaveMsg do quadro atual, obtive %#v", cmd())
	}
}
