// Package ui implementa o modelo Bubble Tea do kanban.
// Regra: nenhuma I/O dentro de Update — alterações pedem um SaveMsg e o main
// (ou os testes) decide o que fazer com ele.
package ui

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"kanban/internal/store"
)

type editMode int

const (
	modeBrowse editMode = iota
	modeNew
	modeEdit
	modeConfirmDelete
	modeHelp
)

// maxUndo limita a pilha de desfazer; cada entrada é um snapshot do quadro.
const maxUndo = 50

// SaveMsg é emitido quando o board mudou e precisa ser gravado em Path.
// Quem roda o programa (main ou teste) decide como/ser gravar.
type SaveMsg struct {
	Board *store.Board
	Path  string
}

// saveDst recebe cada SaveMsg direto do Update — o Bubble Tea 1.3 não expõe
// ao main os Cmds devolvidos pelo Update, então main registra aqui o caminho
// até o seu gravador (via SetSaveDst). Fica atrás de atomic porque é escrito
// por outra goroutine que a do Update.
var saveDst atomic.Pointer[func(SaveMsg)]

// SetSaveDst registra o destino das gravações (nil desliga).
func SetSaveDst(dst func(SaveMsg)) {
	if dst == nil {
		saveDst.Store(nil)
		return
	}
	saveDst.Store(&dst)
}

type snapshot struct {
	cards     [][]string // cópia dos cards de cada coluna
	col, card int
}

type Model struct {
	Board *store.Board
	Path  string

	col    int   // coluna selecionada, 0..2
	card   int   // índice do card focado dentro da coluna
	scroll []int // topo da janela visível de cada coluna

	mode       editMode
	editingIdx int // card em edição (índice); -1 quando é card novo
	ti         textinput.Model

	undo []snapshot
	redo []snapshot

	// dirty indica uma alteração ainda não gravada; SaveErr é o último erro
	// de gravação reportado de volta pelo main (via SaveResultMsg).
	dirty bool
	// Changed fica true desde a primeira alteração da sessão. Serve para
	// detectar gravações fora de ordem (a alteração mais recente pode ter
	// sido gravada antes de uma anterior, que "vence" e deixa o disco velho).
	Changed bool
	SaveErr error
	// BackedUp avisa que o último save encontrou o arquivo alterado por
	// outra instância e guardou aquela versão em board.json.bak.
	BackedUp bool

	width       int
	visibleRows int
}

// SaveResultMsg devolve ao Model o desfecho de uma gravação (erro ou conflito).
type SaveResultMsg struct {
	Err        error
	Conflicted bool
}

func New(board *store.Board, path string) Model {
	ti := textinput.New()
	ti.CharLimit = 120
	ti.Prompt = "" // o textinput já traz "> " de fábrica; aqui não queremos prompt
	return Model{
		Board: board, Path: path,
		mode: modeBrowse, editingIdx: -1,
		scroll:      make([]int, len(board.Columns)),
		ti:          ti,
		width:       80,
		visibleRows: 18,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) cards(c int) []string { return m.Board.Columns[c].Cards }

// clampCard ajusta o cursor quando a coluna mudou ou perdeu cards.
func (m *Model) clampCard() {
	n := len(m.cards(m.col))
	if n == 0 {
		m.card = 0
		return
	}
	if m.card >= n {
		m.card = n - 1
	}
}

// clampCardIn limita o cursor à coluna c (usada ao restaurar snapshots).
func (m *Model) clampCardIn(c int) {
	n := len(m.cards(c))
	if n == 0 {
		m.card = 0
	} else {
		m.card = min(m.card, n-1)
	}
}

// snapshot copia o estado atual do quadro + cursor para a pilha de undo.
func (m *Model) snapshot() snapshot {
	cards := make([][]string, len(m.Board.Columns))
	for i, c := range m.Board.Columns {
		cards[i] = append([]string{}, c.Cards...)
	}
	return snapshot{cards: cards, col: m.col, card: m.card}
}

// pushUndo grava o estado ANTES da próxima alteração e limpa o redo.
func (m *Model) pushUndo() {
	m.undo = append(m.undo, m.snapshot())
	if len(m.undo) > maxUndo {
		m.undo = m.undo[len(m.undo)-maxUndo:]
	}
	m.redo = nil
}

// restore aplica um snapshot ao board.
func (m *Model) restore(s snapshot) {
	for i := range m.Board.Columns {
		if i < len(s.cards) {
			m.Board.Columns[i].Cards = append([]string{}, s.cards[i]...)
		}
	}
	m.col = min(max(s.col, 0), len(m.Board.Columns)-1)
	m.card = s.card
	m.clampCard()
}

// undoStep desfaz a última alteração; devolve false se não havia nada.
func (m *Model) undoStep() bool {
	if n := len(m.undo); n > 0 {
		after := m.snapshot()
		s := m.undo[n-1]
		m.undo = m.undo[:n-1]
		m.redo = append(m.redo, after)
		m.restore(s)
		return true
	}
	return false
}

// redoStep refaz a última alteração desfeita.
func (m *Model) redoStep() bool {
	if n := len(m.redo); n > 0 {
		after := m.snapshot()
		s := m.redo[n-1]
		m.redo = m.redo[:n-1]
		m.undo = append(m.undo, after)
		m.restore(s)
		return true
	}
	return false
}

// changed marca o quadro como sujo e pede a gravação.
func (m Model) changed() (tea.Model, tea.Cmd) {
	m.dirty = true
	msg := SaveMsg{Board: m.Board, Path: m.Path}
	if sink := saveDst.Load(); sink != nil {
		(*sink)(msg)
	}
	return m, func() tea.Msg { return msg }
}

// insertCard insere title na posição idx da coluna col e leva o cursor junto.
func (m *Model) insertCard(col, idx int, title string) {
	c := &m.Board.Columns[col]
	idx = min(max(idx, 0), len(c.Cards))
	c.Cards = append(c.Cards, "")
	copy(c.Cards[idx+1:], c.Cards[idx:])
	c.Cards[idx] = title
	m.col, m.card = col, idx
}

// moveCard move o card focado para a posição pos (0..len) da coluna target.
func (m *Model) moveCard(target, pos int) {
	if target < 0 || target >= len(m.Board.Columns) {
		return
	}
	src := &m.Board.Columns[m.col]
	if m.card >= len(src.Cards) {
		return
	}
	if target == m.col && pos == m.card {
		return
	}
	title := src.Cards[m.card]
	src.Cards = append(src.Cards[:m.card], src.Cards[m.card+1:]...)
	m.insertCard(target, pos, title)
}

// moveFocusedToColumn manda o card para o fim da coluna target; a coluna
// atual reordenada fica intocada.
func (m *Model) moveFocusedToColumn(target int) {
	if target == m.col || target < 0 || target >= len(m.Board.Columns) {
		return
	}
	m.moveCard(target, len(m.Board.Columns[target].Cards))
}

// shiftCard reordena o card focado em delta posições dentro da coluna.
// Nos extremos não faz nada (a ação não tem efeito).
func (m *Model) shiftCard(delta int) bool {
	n := len(m.cards(m.col))
	if m.card+delta < 0 || m.card+delta >= n {
		return false
	}
	m.moveCard(m.col, m.card+delta)
	return true
}

// deleteCard remove o card focado.
func (m *Model) deleteCard() bool {
	c := &m.Board.Columns[m.col]
	if m.card >= len(c.Cards) {
		return false
	}
	c.Cards = append(c.Cards[:m.card], c.Cards[m.card+1:]...)
	m.clampCard()
	return true
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SaveResultMsg:
		m.SaveErr = msg.Err
		if msg.Err == nil {
			m.dirty = false
			m.BackedUp = m.BackedUp || msg.Conflicted
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.visibleRows = max(3, min(msg.Height-6, 50))
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeConfirmDelete {
			return m.updateConfirmDelete(msg)
		}
		if m.mode == modeHelp {
			m.mode = modeBrowse // qualquer tecla fecha a ajuda
			return m, nil
		}
		if m.mode != modeBrowse {
			return m.updateEditor(msg)
		}
		return m.updateBrowse(msg)
	}
	return m, nil
}

// columnForDigit mapeia 1..n para colunas configuradas por índice fixo
// (mesmo quadro com 4+ colunas editado à mão). Cor atual/vazia = no-op.
func (m Model) columnForDigit(d byte) (int, bool) {
	target := int(d - '1')
	if target == m.col || target >= len(m.Board.Columns) || len(m.cards(m.col)) == 0 {
		return 0, false
	}
	return target, true
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "left":
		m.col = max(m.col-1, 0)
		m.clampCard()
	case "right":
		m.col = min(m.col+1, len(m.Board.Columns)-1)
		m.clampCard()
	case "up":
		m.card = max(m.card-1, 0)
	case "down":
		if n := len(m.cards(m.col)); n > 0 {
			m.card = min(m.card+1, n-1)
		}
	case "1", "2", "3":
		target, ok := m.columnForDigit(msg.String()[0])
		if !ok {
			return m, nil
		}
		m.pushUndo()
		m.moveFocusedToColumn(target)
		m.scroll[target] = max(len(m.Board.Columns[target].Cards)-1, 0) // fim da lista: card novo à vista
		return m.changed()
	case "J", "K":
		delta := 1
		if msg.String() == "K" {
			delta = -1
		}
		m.pushUndo()
		if !m.shiftCard(delta) {
			m.undo = m.undo[:len(m.undo)-1] // nos extremos não altera: sem undo/save
			return m, nil
		}
		return m.changed()
	case "d":
		if len(m.cards(m.col)) == 0 {
			return m, nil
		}
		m.mode = modeConfirmDelete
		return m, nil
	case "u", "ctrl+z":
		if m.undoStep() {
			return m.changed()
		}
	case "ctrl+y":
		if m.redoStep() {
			return m.changed()
		}
	case "?":
		m.mode = modeHelp
		return m, nil
	case "n":
		m.mode = modeNew
		m.editingIdx = -1
		m.ti.SetValue("")
		m.ti.Focus()
		return m, nil
	case "enter":
		cards := m.cards(m.col)
		if len(cards) == 0 {
			return m, nil
		}
		m.mode = modeEdit
		m.editingIdx = m.card
		m.ti.SetValue(cards[m.card])
		m.ti.CursorEnd()
		m.ti.Focus()
		return m, nil
	}
	return m, nil
}

// updateConfirmDelete: d de novo apaga, qualquer outra tecla cancela.
func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeBrowse
	if msg.String() != "d" {
		return m, nil
	}
	m.pushUndo()
	if !m.deleteCard() {
		m.undo = m.undo[:len(m.undo)-1]
		return m, nil
	}
	return m.changed()
}

// updateEditor trata teclas enquanto um título está sendo digitado.
// Enter confirma (trim; vazio = descarta), Esc cancela sem tocar o board,
// qualquer outra tecla vai para o textinput — por isso letras nunca
// colidem com atalhos aqui (padrão da skill go-sqlite-tui-apps).
func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	closeEditor := func() Model {
		m.ti.Blur()
		m.mode = modeBrowse
		m.editingIdx = -1
		return m
	}
	switch msg.String() {
	case "esc":
		return closeEditor(), nil
	case "enter":
		title := strings.TrimSpace(m.ti.Value())
		c := &m.Board.Columns[m.col]
		switch {
		case title == "":
			return closeEditor(), nil
		case m.mode == modeNew:
			m.pushUndo()
			c.Cards = append(c.Cards, title)
			m.card = len(c.Cards) - 1
		case m.mode == modeEdit:
			if m.editingIdx < 0 || m.editingIdx >= len(c.Cards) {
				return closeEditor(), nil
			}
			if c.Cards[m.editingIdx] == title {
				return closeEditor(), nil // nada mudou: sem undo nem save
			}
			m.pushUndo()
			c.Cards[m.editingIdx] = title
			m.card = m.editingIdx
		default:
			return closeEditor(), nil
		}
		return closeEditor().changed()
	default:
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
}

// ---- estilos ----
var (
	colInactive   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	colActive     = colInactive.BorderForeground(lipgloss.Color("205"))
	header        = lipgloss.NewStyle().Bold(true)
	cardFocused   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dim           = lipgloss.NewStyle().Faint(true)
	warn          = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	confirmStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	confirmStyleB = confirmStyle.Bold(true)
)

// truncate corta pela LARGURA EM CÉLULAS (acentos pt-BR ocupam 1, emoji/CJK
// 2) — len() de bytes desalinha a coluna inteira e runes ainda estouram em
// caracteres largos.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	return runewidth.Truncate(s, w, "…")
}

// window devolve [top, top+n) que contenha idx, dentro de [0, total).
func window(top, n, idx, total int) int {
	n = max(n, 1)
	if idx < top {
		top = idx
	}
	if idx >= top+n {
		top = idx - n + 1
	}
	return min(max(top, 0), max(total-n, 0))
}

// columnView desenha uma coluna. outerW é a largura TOTAL da caixa (com borda
// e padding); conteúdo = outerW-4. Cabeçalho sempre mostra o total, então a
// lista pode ser cortada por altura sem esconder informação.
func (m Model) columnView(i, outerW int) string {
	col := m.Board.Columns[i]
	active := i == m.col && (m.mode == modeBrowse || m.mode == modeConfirmDelete)
	content := outerW - 4

	lines := []string{header.Render(truncate(fmt.Sprintf("%s (%d)", col.Title, len(col.Cards)), content))}
	budget := m.visibleRows - 1 // -1: reserva linha p/ o indicador de corte
	if budget < 1 {
		budget = 1
	}
	// O cursor da coluna ativa sempre precisa caber na janela visível.
	focusIdx := m.card
	if !active && !(m.mode != modeBrowse && i == m.col) {
		focusIdx = m.scroll[i]
	}
	top := window(m.scroll[i], budget, focusIdx, len(col.Cards))
	shown := min(len(col.Cards)-top, budget)

	for j := top; j < top+shown; j++ {
		txt := truncate(col.Cards[j], content-3) // " ▸ " custa 3 células
		if active && j == m.card {
			lines = append(lines, cardFocused.Render(" ▸ "+txt))
		} else {
			lines = append(lines, "   "+txt) // mesmo prefixo de 3 células do foco
		}
	}
	if hidden := len(col.Cards) - shown; hidden > 0 {
		lines = append(lines, dim.Render(fmt.Sprintf("    ↕ %d/%d", top+shown, len(col.Cards))))
	}
	if m.mode != modeBrowse && i == m.col {
		m.ti.Width = max(content-3, 1)
		lines = append(lines, " ✎ "+m.ti.View())
	}
	if active {
		return colActive.Width(outerW).Render(strings.Join(lines, "\n"))
	}
	return colInactive.Width(outerW).Render(strings.Join(lines, "\n"))
}

// footer lista SÓ as teclas válidas agora (atalho morto treina o usuário a
// ignorar a legenda — padrão da skill).
func (m Model) footer() string {
	w := max(m.width, 30)
	if m.SaveErr != nil {
		return warn.Render(truncate("FALHOU SALVAR: "+m.SaveErr.Error()+" — alterações não estão no disco", w))
	}
	switch m.mode {
	case modeNew, modeEdit:
		return dim.Render(truncate("Enter confirma · Esc cancela", w))
	case modeHelp:
		return dim.Render(truncate("←→ coluna · ↑↓ card · Enter editar · n novo · d apagar (dd) · J/K reordenar · 1-3 mover p/ coluna · u desfazer · Ctrl+Y refazer · ? fechar", w))
	case modeConfirmDelete:
		return confirmStyleB.Render(truncate(fmt.Sprintf("Apagar %q? d confirma · qualquer outra tecla cancela", truncate(m.cards(m.col)[m.card], 40)), w))
	}
	parts := []string{"←→ coluna", "↑↓ card", "n novo", "q sair"}
	if len(m.cards(m.col)) > 0 {
		parts = append([]string{"Enter editar", "d apagar", "J/K mover dentro"}, parts...)
	}
	if m.undo != nil && len(m.undo) > 0 {
		parts = append(parts, "u desfazer")
	}
	if len(m.redo) > 0 {
		parts = append(parts, "ctrl+y refazer")
	}
	if n := len(m.Board.Columns); n > 1 {
		parts = append(parts, "1-"+fmt.Sprint(n)+" mover para coluna")
	}
	line := dim.Render(truncate(strings.Join(parts, " · "), w))
	if m.dirty {
		line += " " + warn.Render("• alteração não gravada")
	}
	if m.BackedUp {
		line += " " + warn.Render("• versão anterior em "+m.Path+".bak")
	}
	return line
}

func (m Model) View() string {
	w := max(m.width, 30)
	outer := (w - 6) / 3
	if outer < 12 {
		outer = 12
	}
	cols := make([]string, len(m.Board.Columns))
	for i := range m.Board.Columns {
		cols[i] = m.columnView(i, outer)
	}
	top := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	head := truncate(fmt.Sprintf("kanban · %s · ? ajuda", m.Path), w)
	return head + "\n\n" + top + "\n\n" + m.footer() + "\n"
}
