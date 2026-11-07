// Package store lê e grava o quadro Kanban em um JSON simples e legível.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	ColTodo  = "A Fazer"
	ColDoing = "Fazendo"
	ColDone  = "Feito"

	// MaxFileSize evita travar o app ao abrir um arquivo gigante por engano.
	MaxFileSize = 10 << 20 // 10 MiB
)

// Column é uma coluna do quadro. Cards é fatia não-nil sempre.
type Column struct {
	Title string   `json:"title"`
	Cards []string `json:"cards"`
}

// Board é o quadro inteiro. Suporta N colunas (≥ 1), na ordem em que estão
// gravadas; Default() cria as três fixas A Fazer / Fazendo / Feito.
type Board struct {
	Columns []Column `json:"columns"`

	// ModTime é o mtime do arquivo no momento em que foi lido (zero se não
	// existia). Usado para detectar outra instância/editores gravando por cima.
	ModTime time.Time `json:"-"`
}

// Default retorna um quadro vazio com as três colunas fixas.
func Default() *Board {
	return &Board{Columns: []Column{
		{Title: ColTodo, Cards: []string{}},
		{Title: ColDoing, Cards: []string{}},
		{Title: ColDone, Cards: []string{}},
	}}
}

// Load lê o board de path. Arquivo inexistente não é erro: devolve Default()
// (o arquivo só é criado no primeiro salvamento). JSON inválido ou schema
// diferente do esperado é erro claro — o usuário edita esse arquivo na mão,
// então corrompê-lo deve ser reportado, nunca ignorado.
func Load(path string) (*Board, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", path, err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", path, err)
	}
	if st.Size() > MaxFileSize {
		return nil, fmt.Errorf("%s: arquivo grande demais (%d bytes, limite %d)", path, st.Size(), MaxFileSize)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", path, err)
	}

	var b Board
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("JSON inválido em %s: %w", path, err)
	}
	if len(b.Columns) == 0 {
		return nil, fmt.Errorf("%s: esperava ao menos 1 coluna, achei 0", path)
	}
	seen := make(map[string]bool, len(b.Columns))
	for i := range b.Columns {
		title := b.Columns[i].Title
		if title == "" {
			return nil, fmt.Errorf("%s: coluna %d sem título", path, i+1)
		}
		if seen[title] {
			return nil, fmt.Errorf("%s: título de coluna repetido %q", path, title)
		}
		seen[title] = true
		if b.Columns[i].Cards == nil {
			b.Columns[i].Cards = []string{}
		}
	}
	b.ModTime = st.ModTime()
	return &b, nil
}

// Save grava o board em path de forma atômica: escreve num temporário no
// mesmo diretório, faz fsync e renomeia por cima. Assim, um crash no meio
// da gravação nunca deixa board.json pela metade (o usuário edita esse
// arquivo em outro editor também). Não toca b.ModTime.
func Save(path string, b *Board) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'))
}

// SaveIfUnchanged é o caminho normal de gravação: se o arquivo em disco
// mudou desde o último load/save (outra instância ou editor gravou por cima),
// guarda aquela versão em <path>.bak antes de sobrescrever.
// Retorna conflicted=true quando o backup foi feito.
func SaveIfUnchanged(path string, b *Board) (conflicted bool, err error) {
	if st, err := os.Stat(path); err == nil {
		if !b.ModTime.IsZero() && !st.ModTime().Equal(b.ModTime) {
			if bak, err := os.ReadFile(path); err == nil {
				if err := os.WriteFile(path+".bak", bak, 0o644); err != nil {
					return false, fmt.Errorf("backup %s.bak: %w", path, err)
				}
			}
			conflicted = true
		}
	}
	return conflicted, Save(path, b)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("criar diretório de %s: %w", path, err)
	}
	tmp, err := os.CreateTemp(dir, ".board-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op depois do Rename bem-sucedido
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
