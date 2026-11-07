package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

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

// saver grava pedidos de save EM ORDEM (fila + uma goroutine), detectando se
// outra instância/editor gravou por cima desde a última leitura — nesse caso
// guarda aquela versão em <path>.bak.
type saver struct {
	requests chan ui.SaveMsg
	onDone   func(ui.SaveResultMsg)
	stopped  atomic.Bool

	// owned pelos campos abaixo via mu; result() é chamado depois de stop().
	mu         sync.Mutex
	conflicted bool
	lastErr    error
}

func newSaver(onDone func(ui.SaveResultMsg)) *saver {
	s := &saver{requests: make(chan ui.SaveMsg, 16), onDone: onDone}
	go s.loop()
	return s
}

func (s *saver) loop() {
	for req := range s.requests {
		conf, err := store.SaveIfUnchanged(req.Path, req.Board)
		if err == nil {
			// Registra o novo mtime para a próxima gravação não descobrir
			// um "conflito" consigo mesma.
			if st, statErr := os.Stat(req.Path); statErr == nil {
				req.Board.ModTime = st.ModTime()
			}
		}
		s.mu.Lock()
		s.conflicted = s.conflicted || conf
		s.lastErr = err
		s.mu.Unlock()
		if s.onDone != nil {
			s.onDone(ui.SaveResultMsg{Err: err, Conflicted: conf})
		}
	}
}

// save enfileira um pedido; depois de stop() os pedidos tardios são ignorados
// (o estado final é gravado pelo próprio run).
func (s *saver) save(msg ui.SaveMsg) {
	if s.stopped.Load() {
		return
	}
	s.requests <- msg
}

// stop encerra a goroutine; close espera as gravações pendentes terminarem.
func (s *saver) stop() {
	if s.stopped.CompareAndSwap(false, true) {
		close(s.requests)
	}
}

func (s *saver) result() (conflicted bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conflicted, s.lastErr
}

func run(path string) error {
	board, err := store.Load(path)
	if err != nil {
		return fmt.Errorf("%w\ncorrija o JSON ou aponte --file para outro caminho", err)
	}

	// O Program só existe depois de criado; o callback fecha sobre ele.
	// Bubble Tea 1.3 não expõe ao main os Cmds devolvidos pelo Update, então
	// o modelo entrega cada SaveMsg por ui.SaveDst — o gravador roda fora do
	// loop (em ordem) e devolve o desfecho via p.Send.
	var p *tea.Program
	s := newSaver(func(r ui.SaveResultMsg) { p.Send(r) })
	p = tea.NewProgram(ui.New(board, path), tea.WithAltScreen())
	ui.SetSaveDst(s.save)
	final, err := p.Run()
	s.stop() // espera as gravações pendentes antes de decidir o estado final
	if err != nil {
		return fmt.Errorf("erro na interface: %w", err)
	}

	m := final.(ui.Model)
	conflicted, saveErr := s.result()
	if saveErr != nil || m.SaveErr != nil || m.Changed {
		// Rede de segurança: grava o estado final (o quadro na tela é a
		// verdade). Cobre também o caso de a última gravação ter falhado.
		conf, err := store.SaveIfUnchanged(path, m.Board)
		if conf {
			conflicted = true
		}
		if err == nil {
			saveErr = nil // o estado final foi gravado
		} else {
			return fmt.Errorf("falhou SALVAR: %w\nseu quadro NÃO foi gravado", err)
		}
	}
	if conflicted {
		fmt.Fprintln(os.Stderr, "aviso: o arquivo foi alterado por outro processo; a versão anterior ficou em", path+".bak")
	}
	if saveErr != nil {
		return fmt.Errorf("falhou SALVAR: %w\nseu quadro NÃO foi gravado", saveErr)
	}
	fmt.Println("salvo em", path)
	return nil
}

func main() {
	path := flag.String("file", defaultPath(), "caminho do JSON do quadro")
	flag.Parse()

	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
