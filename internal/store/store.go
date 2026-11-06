// Package store lê e grava o quadro Kanban em um JSON simples e legível.
package store

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