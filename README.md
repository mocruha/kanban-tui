# kanban

Quadro Kanban pessoal no terminal (Go + Bubble Tea). 3 colunas fixas —
A Fazer / Fazendo / Feito — cards com só título, salvos em JSON legível
que você pode editar em qualquer editor.

## Uso
    kanban                    # usa ~/.local/share/kanban/board.json
    kanban --file meukan.json

## Teclas
    ← →        trocar coluna        1 2 3   mover o card p/ coluna
    ↑ ↓        mover cursor         n       novo card (fim da coluna)
    Enter      editar título        d       apagar
    u          desfazer apagar      q/Esc   sair (salva automaticamente)

Durante a digitação: Enter confirma, Esc cancela, espaço no título é normal.

## Formato
`{"columns":[{"title":"A Fazer","cards":["..."]}, ...]}` — 3 colunas, sempre.
JSON corrompido -> o app recusa abrir e mostra o caminho; edite/conserte na mão.