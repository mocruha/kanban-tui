// Package ui implementa o modelo Bubble Tea do kanban.
// Regra: nenhuma I/O dentro de Update — o salvamento acontece no main,
// depois que o programa termina.
package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kanban/internal/store"
)

type editMode int

const (
	modeBrowse editMode = iota
	modeNew
	modeEdit
)

type undoEntry struct {
	col, idx int
	title    string
}

type Model struct {
	Board *store.Board
	Path  string

	col  int // coluna selecionada, 0..2
	card int // índice do card focado dentro da coluna

	mode       editMode
	editingIdx int // card em edição (índice); -1 quando é card novo
	ti         textinput.Model

	undo *undoEntry

	width       int
	visibleRows int
}

func New(board *store.Board, path string) Model {
	ti := textinput.New()
	ti.CharLimit = 120
	ti.Prompt = "" // o textinput já traz "> " de fábrica; aqui não queremos prompt
	return Model{
		Board: board, Path: path,
		mode: modeBrowse, editingIdx: -1,
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

// moveCard move o card focado para a coluna target (append no fim) e leva o
// cursor junto. Mesmo índice de origem fora do intervalo: no-op defensivo.
func (m *Model) moveCard(target int) {
	if target == m.col || target < 0 || target >= len(m.Board.Columns) {
		return
	}
	src := &m.Board.Columns[m.col]
	if m.card >= len(src.Cards) {
		return
	}
	title := src.Cards[m.card]
	src.Cards = append(src.Cards[:m.card], src.Cards[m.card+1:]...)
	dst := &m.Board.Columns[target]
	dst.Cards = append(dst.Cards, title)
	m.col = target
	m.card = len(dst.Cards) - 1
	m.undo = nil
}

// deleteCard remove o card focado e guarda undo de 1 nível.
func (m *Model) deleteCard() {
	c := &m.Board.Columns[m.col]
	if m.card >= len(c.Cards) {
		return
	}
	m.undo = &undoEntry{col: m.col, idx: m.card, title: c.Cards[m.card]}
	c.Cards = append(c.Cards[:m.card], c.Cards[m.card+1:]...)
	m.clampCard()
}

// restoreUndo reinsere o último card apagado na posição original (clampada).
func (m *Model) restoreUndo() {
	if m.undo == nil {
		return
	}
	c := &m.Board.Columns[m.undo.col]
	i := min(m.undo.idx, len(c.Cards))
	c.Cards = append(c.Cards[:i], append([]string{m.undo.title}, c.Cards[i:]...)...)
	m.col, m.card = m.undo.col, i
	m.undo = nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.visibleRows = max(3, min(msg.Height-6, 50))
		return m, nil

	case tea.KeyMsg:
		if m.mode != modeBrowse {
			return m.updateEditor(msg)
		}
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
		case "1", "2", "3":
			m.moveCard(int(msg.String()[0] - '1'))
		case "d":
			m.deleteCard()
		case "u":
			m.restoreUndo()
		case "down":
			if n := len(m.cards(m.col)); n > 0 {
				m.card = min(m.card+1, n-1)
			}
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
	return m, nil
}

// updateEditor trata teclas enquanto um título está sendo digitado.
// Enter confirma (trim; vazio = descarta), Esc cancela sem tocar o board,
// qualquer outra tecla vai para o textinput — por isso letras nunca
// colidem com atalhos aqui (padrão da skill go-sqlite-tui-apps).
func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ti.Blur()
		m.mode = modeBrowse
		m.editingIdx = -1
		return m, nil
	case "enter":
		title := strings.TrimSpace(m.ti.Value())
		c := &m.Board.Columns[m.col]
		if title != "" {
			switch m.mode {
			case modeNew:
				c.Cards = append(c.Cards, title)
				m.card = len(c.Cards) - 1
			case modeEdit:
				if m.editingIdx >= 0 && m.editingIdx < len(c.Cards) {
					c.Cards[m.editingIdx] = title
					m.card = m.editingIdx
				}
			}
			m.undo = nil
		}
		m.ti.Blur()
		m.mode = modeBrowse
		m.editingIdx = -1
		return m, nil
	default:
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
}
// ---- estilos ----
var (
	colInactive = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	colActive   = colInactive.BorderForeground(lipgloss.Color("205"))
	header      = lipgloss.NewStyle().Bold(true)
	cardFocused = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dim         = lipgloss.NewStyle().Faint(true)
)

// truncate corta por RUNES (acentos pt-BR ocupam 1 célula, 2 bytes —
// len() de bytes desalinha a coluna inteira; padrão da skill).
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= w {
		return s
	}
	return string([]rune(s)[:w-1]) + "…"
}

// columnView desenha uma coluna. outerW é a largura TOTAL da caixa (com borda
// e padding); conteúdo = outerW-4. Cabeçalho sempre mostra o total, então a
// lista pode ser cortada por altura sem esconder informação.
func (m Model) columnView(i, outerW int) string {
	col := m.Board.Columns[i]
	active := i == m.col && m.mode == modeBrowse
	content := outerW - 4

	lines := []string{header.Render(fmt.Sprintf("%s (%d)", col.Title, len(col.Cards)))}
	shown := min(len(col.Cards), m.visibleRows-1) // -1: reserva linha p/ "(+N)"
	for j := 0; j < shown; j++ {
		txt := truncate(col.Cards[j], content-3) // " ▸ " custa 3 células
		if active && j == m.card {
			lines = append(lines, cardFocused.Render(" ▸ "+txt))
		} else {
			lines = append(lines, "    "+txt)
		}
	}
	if hidden := len(col.Cards) - shown; hidden > 0 {
		lines = append(lines, dim.Render(fmt.Sprintf("    (+%d)", hidden)))
	}
	if m.mode != modeBrowse && i == m.col {
		m.ti.Width = content - 3
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
	if m.mode != modeBrowse {
		return dim.Render(truncate("Enter confirma · Esc cancela", max(m.width, 30)))
	}
	parts := []string{"←→ coluna", "↑↓ card", "1-3 mover card", "n novo", "q sair"}
	if len(m.cards(m.col)) > 0 {
		parts = append(parts[:3], append([]string{"Enter editar", "d apagar"}, parts[3:]...)...)
	}
	if m.undo != nil {
		parts = append(parts, "u desfazer")
	}
	return dim.Render(truncate(strings.Join(parts, " · "), max(m.width, 30)))
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
	head := fmt.Sprintf("kanban · %s · salva ao sair (q)", m.Path)
	return head + "\n\n" + top + "\n\n" + m.footer() + "\n"
}