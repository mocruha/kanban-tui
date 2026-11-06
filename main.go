package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"kanban/internal/store"
	"kanban/internal/ui"
)

// defaultPath segue XDG: ~/.local/share/kanban/board.json (ou $XDG_DATA_HOME).
func defaultPath() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "kanban", "board.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "kanban", "board.json")
}

func main() {
	path := flag.String("file", defaultPath(), "caminho do JSON do quadro")
	flag.Parse()

	board, err := store.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		fmt.Fprintln(os.Stderr, "corrija o JSON ou aponte --file para outro caminho")
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(board, *path), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro na interface:", err)
		os.Exit(1)
	}

	// Salvamento único no fim: Update nunca faz I/O (padrão bubbletea).
	// SIGINT passa pelo DefaultSignalHandler -> Run retorna -> salva também.
	if err := store.Save(*path, final.(ui.Model).Board); err != nil {
		fmt.Fprintln(os.Stderr, "falhou SALVAR:", err)
		fmt.Fprintln(os.Stderr, "seu quadro NÃO foi gravado")
		os.Exit(1)
	}
	fmt.Println("salvo em", *path)
}