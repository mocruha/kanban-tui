// Package store lê e grava o quadro Kanban em um JSON simples e legível.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ColTodo  = "A Fazer"
	ColDoing = "Fazendo"
	ColDone  = "Feito"
)

// Column é uma coluna do quadro. Cards é fatia não-nil sempre.
type Column struct {
	Title string   `json:"title"`
	Cards []string `json:"cards"`
}

// Board é o quadro inteiro. Sempre exatamente 3 colunas, na ordem
// A Fazer / Fazendo / Feito.
type Board struct {
	Columns []Column `json:"columns"`
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
// (o arquivo só é criado no primeiro salvamento). JSON inválido ou número de
// colunas errado é erro claro — o usuário edita esse arquivo na mão, então
// corrompê-lo deve ser reportado, nunca ignorado.
func Load(path string) (*Board, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", path, err)
	}
	var b Board
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("JSON inválido em %s: %w", path, err)
	}
	if len(b.Columns) != 3 {
		return nil, fmt.Errorf("%s: esperava 3 colunas, achei %d", path, len(b.Columns))
	}
	for i := range b.Columns {
		if b.Columns[i].Cards == nil {
			b.Columns[i].Cards = []string{}
		}
	}
	return &b, nil
}

// Save grava o board em path de forma atômica: escreve num temporário no
// mesmo diretório, faz fsync e renomeia por cima. Assim, um crash no meio
// da gravação nunca deixa board.json pela metade (o usuário edita esse
// arquivo em outro editor também).
func Save(path string, b *Board) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("criar diretório de %s: %w", path, err)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".board-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op depois do Rename bem-sucedido
	if _, err := tmp.Write(append(data, '\n')); err != nil {
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