# kanban

Quadro Kanban pessoal no terminal (Go + Bubble Tea). Por padrão, 3 colunas —
A Fazer / Fazendo / Feito — empilhadas de cima para baixo (A Fazer no topo,
Feito embaixo), com os cards de cada coluna também empilhados, um por linha
para o título caber por extenso. Salvos em JSON legível que você pode editar
em qualquer editor.

## Instalação
    make install      # compila e instala em ~/local/bin/kanban
    make test         # testes (com -race)
    make vet          # análise estática

Ou, direto: `go build -o kanban .`

## Uso
    kanban                    # usa ~/.local/share/kanban/board.json
    kanban --file meukan.json

## Teclas
    ← →        trocar coluna        1 … 9   mover o card p/ coluna
    ↑ ↓        mover cursor         J / K   mover o card dentro da coluna
    Enter      editar título        n       novo card (fim da coluna)
    d d        apagar (2× confirma) u       desfazer
    Ctrl+Y     refazer              ?       ajuda completa
    q/Esc      sair

Durante a digitação: Enter confirma, Esc cancela, espaço no título é normal.
Ctrl+Z também desfaz. A lista rola sozinha para manter o card focado à vista.

## Salvamento
Cada alteração é gravada em disco na hora (gravação atômica: temporário +
fsync + rename no mesmo diretório). Se a gravação falhar, o rodapé mostra o
erro e o app tenta de novo ao sair. Se **outra** instância (ou você mesmo pelo
editor) alterou o arquivo durante a sessão, a versão do disco é guardada em
`board.json.bak` antes de ser sobrescrita.

## Formato
`{"columns":[{"title":"A Fazer","cards":["..."]}, ...]}` — N colunas
(3 por padrão), empilhadas de cima para baixo: a primeira do JSON fica no
topo e a última embaixo; dentro de cada coluna os cards vêm na ordem da
lista, um por linha. Os títulos podem ser renomeados à mão e novas colunas
podem ser adicionadas. O app recusa abrir (mostra o caminho e o motivo) se o
JSON estiver corrompido, não houver coluna, houver título vazio/repetido ou o
arquivo passar de 10 MiB; conserte na mão ou aponte `--file` para outro
caminho.
