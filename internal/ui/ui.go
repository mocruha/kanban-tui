// Package ui implementa o modelo Bubble Tea do kanban.
// Regra: nenhuma I/O dentro de Update — o salvamento acontece no main,
// depois que o programa termina.
package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
		case "down":
			if n := len(m.cards(m.col)); n > 0 {
				m.card = min(m.card+1, n-1)
			}
		}
		return m, nil
	}
	return m, nil
}

// placeholder temporário: as tarefas 6-9 substituem/adicionam.
func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
func (m Model) View() string                                     { return "" }