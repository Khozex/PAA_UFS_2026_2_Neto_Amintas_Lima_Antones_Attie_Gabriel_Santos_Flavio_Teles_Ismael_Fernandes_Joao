# AV2 - Trilha C - Grafos e conhecimento

Na AV1, cada operação da API era um documento isolado e o ranking usava TF-IDF.
Aqui o corpus também vira um grafo, e três algoritmos (BFS, DFS e Dijkstra)
fazem duas coisas:

1. achar operações ligadas aos resultados da busca ;
2. dizer em que ordem chamar as operações, como, por exemplo, listar antes de apagar.

Funcionalidade em `searchEngine/graph/`, com flags de uso em `searchEngine/main.go`.

## Estrutura em grafo do corpus

**Vértices**: unidade de informação no grafo que pode ser um recurso ou operação

- *Recurso*: um pedaço do path, como `/repos/{owner}/{repo}/releases`;
- *Operação*: método + path, como `DELETE /repos/{owner}/{repo}/releases/{release_id}`.
  Cada operação é um documento do corpus.

**Arestas**: Ligam dois vértices e têm direção e um peso.

| Aresta | De → Para | Peso | Quer dizer |
|---|---|---|---|
| `Contains` | recurso → recurso filho | 1 | `/repos` contém `/repos/{owner}` |
| `Within` | recurso filho → recurso pai | 1 | o contrário da operação anterior|
| `Exposes` | recurso → operação | 0,5 | o recurso tem essa operação |
| `ExposedBy` | operação → recurso | 0,5 | o contrário |
| `DependsOn` | operação → operação | 1 | precisa ser chamada antes (ex.: `DELETE …/releases/{id}` precisa de `GET …/releases`, que dá o `{id}`) |

- **Peso**: é a "distância" entre dois itens. Quanto menor, mais perto. Duas
  operações do mesmo recurso ficam a custo 1 (0,5 + 0,5). Os valores 1 e 0,5 foram
  escolhidos trivialmente, e não aprendidos dos dados.
- **Direção**: as quatro primeiras arestas existem nos dois sentidos, então dá
  para andar nos dois. Só `DependsOn` tem um sentido que importa: sai da
  operação e vai para o que ela exige antes.
- **Caminho**: uma sequência de vértices ligados por arestas. Por exemplo
  `DELETE …/releases/{id}` → recurso `…/releases/{id}` → `PATCH …/releases/{id}`
  mostra que as duas operações mexem no mesmo recurso. O custo do caminho é a
  soma dos pesos.

### Grafo × heurística de busca por texto

| | Busca por texto com TF-IDF | Busca no grafo |
|---|---|---|
| Responde | "quais operações falam disso?" | "o que está ligado a isso?" |
| Bom para | achar pelas palavras da pergunta | achar vizinhos que não repetem as palavras; ordenar chamadas |
| Limitação | trata cada operação sozinha | precisa de um ponto de partida vindo da busca por texto |

Exemplo (considerando corpus de teste com 74 operações): para "delete a release", o TF-IDF
devolve o `DELETE` e o `GET` do release. O grafo, a partir deles, acrescenta o
`PATCH`, os `assets` e a lista `GET/POST …/releases`. E o plano de chamadas sai
assim: `1. GET …/releases` → `2. DELETE …/releases/{release_id}`. Essa ordem
não sai do ranking por texto.

### Como usar

```
go run ./searchEngine -query "delete a release" -k 2 -expand bfs -hops 2
go run ./searchEngine -query "delete a release" -k 2 -expand dijkstra -budget 2
go run ./searchEngine -query "delete a release" -plan
```

- `-expand bfs|dijkstra` mostra até 10 operações relacionadas.
- `-plan` mostra a ordem de chamadas do primeiro resultado.
- `-hops` (padrão 3) limita a BFS; `-budget` (padrão 3) limita o Dijkstra.

Obs.: A BFS conta arestas (operação → recurso → operação = 2). O Dijkstra soma
  pesos (mesmo recurso = 1). Por isso `-hops 2` e `-budget 2` não devolvem
  exatamente o mesmo conjunto.

## Especificação do problema

### Problema 1: achar evidências conectadas

- **Entrada**: o grafo; a consulta; as operações que a busca por texto
  devolveu (as "sementes", top-k); o método (BFS ou Dijkstra); um limite de
  saltos `H` (BFS) ou de custo `B` (Dijkstra); e um máximo de resultados `L`.
- **Saída**: lista de operações, sem repetir as sementes, da mais próxima à mais
  distante.
- **Restrições**: pesos nunca negativos; por padrão não se usa a aresta `DependsOn`.
- **Objetivo**: trazer as operações mais próximas das sementes.
- **Corretude**:
  - No BFS, a lista tem todas as operações a até `H` arestas de alguma semente,
    ordenadas pelo número de arestas;
  - No Dijkstra, a lista tem todas as operações com custo mínimo até `B` a partir
    de alguma semente, ordenadas por esse custo, e o caminho mostrado tem
    exatamente esse custo;
  - Em caso de empate, vale a ordem alfabética do ID.

### Problema 2: ordem das chamadas

- **Entrada**: o grafo e a operação-alvo.
- **Saída**: lista de operações.
- **Corretude**: a lista termina no alvo, tem só o alvo e o que ele
  exige (direta ou indiretamente), e cada operação aparece depois das que ela
  exige. Se as dependências formarem um ciclo, a resposta é um erro, porque não
  existe ordem possível.

### Corpus, consultas e parâmetros

| Item | Valor |
|---|---|
| Corpus | `data/processed/docs.jsonl`: 1224 operações da GitHub REST API v1.1.4 |
| Representação | Um documento por operação (método + path); vértices e arestas como acima |
| Tamanho do grafo | `V = R + O` vértices (R recursos, O = 1224 operações); `E = 2(R−1) + 2·O + D` arestas (D dependências) |
| Consultas | As 10 do gabarito `data/queries.json`, mais consultas livres |
| Sementes | k = 5 no CLI, k = 3 na interface; `-preset 2` |
| Expansão | H = 3, B = 3, L = 10 |
| Limite de contexto | Por quantidade de operações (`k`, `L`) e, na interface, por caracteres (`-max_chars`). O grafo não conta tokens; isso fica com a camada RAG |

### Baseline

1. **Busca por texto da AV1** (TF-IDF, `-preset 2`, top-k). A comparação é
   "só o top-k" contra "top-k + expansão pelo grafo".
2. **Expansão simples**: para cada semente, pegar as operações com o mesmo
   começo de path, sem grafo e sem pesos.
3. **Busca linear** (`-preset 1`): serve para conferir que o ranking está certo, e chega no mesmo resultado que a indexada. Não substitui a busca em grafo.

Para o Problema 2 não há linha de base na AV1, pois o TF-IDF não ordena
chamadas. A referência é a regra "lista antes do item".

### Casos de borda

| Caso | O que acontece | Teste |
|---|---|---|
| Corpus vazio | Grafo vazio; as buscas devolvem lista vazia, sem erro | `TestEmptyAndDisconnected` |
| Consulta sem resultado | Sem sementes, nada a expandir; devolve vazio | `TestExpand` |
| Semente que não está no grafo | É ignorada | `TestEmptyAndDisconnected` |
| Empate | Desempata por ID (e por índice do vértice no Dijkstra); o resultado deve ser sempre o mesmo | `TestBuildStructure` |
| Documento duplicado (mesmo método + path) | Vira uma operação só | `TestDuplicateDocumentCollapses` |
| Limite excedido | Trunca a lista, mantendo os mais próximos | `TestBFSDepthLimitAndDijkstraBudget`, `TestExpand` |
| Grafo desconexo / sem caminho | Distância infinita; `Path` avisa que não há caminho | `TestEmptyAndDisconnected` |
| Peso negativo ou inválido | `AddEdge` recusa | `TestAddEdgeRejectsInvalid` |
| Ciclo de dependência | `TopoOrder` e `Plan` devolvem erro. No grafo montado pelo `Build` não acontece, só em grafos feitos de maneira arbitrária | `TestDFSDetectsCycle` |
| Operação sem pré-requisito | O plano tem só ela | `TestPlanOrdersPrerequisitesFirst` |


## Algoritmos e provas de corretude

### Estruturas de dados

- **Grafo**: lista de vértices e, para cada vértice, a lista de arestas que saem
  dele (`Adj`). Cada aresta guarda destino, peso e tipo.
- **Filtro**: função que diz quais arestas podem ser usadas (por exemplo, só
  `DependsOn`).
- **BFS**: uma fila, e vetores `depth` (nº de arestas desde a semente) e
  `parent` (de onde veio).
- **DFS**: uma pilha, e uma cor por vértice (não visto, na pilha, terminado).
- **Dijkstra**: vetores `Dist` (menor custo até agora) e `Prev` (de onde veio), e
  uma fila de prioridade (heap) ordenada por custo.

### BFS

Anda por "camadas": primeiro as sementes, depois os vizinhos delas, e assim por
diante. Ignora pesos e minimiza o número de arestas.

```
BFS(grafo, sementes, H):
  depth[v] = -1 para todo v
  para cada semente s: depth[s] = 0; coloca s na fila
  enquanto a fila não está vazia:
    u = tira o primeiro da fila
    se depth[u] == H: continue          //não passa do limite
    para cada aresta u → v permitida:
      se depth[v] == -1:                //ainda não visitado
        depth[v] = depth[u] + 1; parent[v] = u; coloca v na fila
```

**Prova de corretude:** A fila sempre está em ordem de `depth`, e do primeiro
ao último a diferença é no máximo 1. Dá para mostrar por indução: as sementes
têm `depth` 0. Se um vértice está a `d+1` arestas, o vizinho dele a `d` arestas
sai da fila antes e o descobre com `d+1`. Ele não pode ter sido descoberto com um
valor menor, senão existiria um caminho mais curto. Parar em `H` só deixa de
expandir os vértices em `H`, o que não muda os que estão até `H`.

### DFS

Vai o mais fundo que puder por um caminho e só volta quando não há mais para
onde ir. Aqui ela é feita com pilha (sem recursão). Serve para achar ciclos
e para ordenar dependências.

```
DFS(grafo, sementes):
  para cada semente s ainda não vista:
    empilha s (cor: "na pilha")
    enquanto a pilha não está vazia:
      u = topo
      se u não tem mais arestas: marca u como terminado; anota u em "Post"; desempilha
      senão, para a próxima aresta u → v:
        v não visto:     empilha v
        v "na pilha":    achou um ciclo
Plan(alvo) = "Post" da DFS que parte do alvo seguindo só DependsOn
```

**Prova de corretude**
- Ciclo: voltar para um vértice que ainda está na pilha significa que há um
  caminho dele até `u` e uma aresta de `u` de volta a ele, ou seja, um ciclo.
  Se existe um ciclo, a DFS acaba passando por uma aresta assim, então ela não
  deixa ciclo escapar.
- Ordem: se `u` exige `v` e não há ciclo, `v` termina **antes** de `u` (ou já
  tinha terminado, ou é visitado dentro de `u`). Como `Post` lista os vértices
  na ordem em que terminam, os pré-requisitos vêm primeiro e o alvo por
  último. É isso que o `Plan` devolve.
- A DFS a partir do alvo visita só o que ele exige, direta ou indiretamente.


### Dijkstra

Sempre escolhe o vértice não fechado mais barato, "fecha" esse vértice e
atualiza o custo dos vizinhos (o chamado *relaxamento*: se passar por aqui sai
mais barato, troca).

```
Dijkstra(grafo, sementes, B):
  Dist[v] = infinito para todo v
  para cada semente s: Dist[s] = 0; coloca s no heap
  enquanto o heap não está vazio:
    u = tira o de menor custo
    se u já está fechado: continue
    fecha u
    para cada aresta u → v permitida, com peso w:
      novo = Dist[u] + w
      se novo > B: continue             (passou do orçamento)
      se novo < Dist[v]: Dist[v] = novo; Prev[v] = u; coloca v no heap
```

**Prova de corretude:** Quando um vértice é fechado, o
`Dist` dele já é o menor possível. Prova por contradição: suponha que `u` foi
fechado com um custo maior que o ótimo. No caminho ótimo até `u` existe um
primeiro vértice `y` ainda não fechado, e o anterior a ele já foi fechado e o
relaxou, então `Dist[y]` está certo. Como nenhum peso é negativo, `y` custa no
máximo o mesmo que `u`, então sairia do heap antes dele. Contradição. Daí
segue:
- os vértices são fechados em ordem crescente de custo;
- várias sementes funcionam como uma única semente ligada a todas por arestas de
  custo 0;
- o orçamento `B` não atrapalha: se um caminho custa até `B`, qualquer pedaço
  inicial dele também custa até `B`, então cortar o que passa de `B` não perde
  nenhum caminho dentro do limite.

### Como foi verificado nos testes

- Dijkstra comparado com o algoritmo de Bellman-Ford (que calcula o mesmo
  resultado de outro jeito) em 300 grafos aleatórios, com pesos de 0 a 3,5.
- BFS e Dijkstra dão o mesmo resultado quando todos os pesos são 1 (200 grafos).
- Em um grafo onde os dois divergem, como 1 aresta de custo 10 contra 3 arestas de custo 1, a BFS escolhe a de menos arestas, o Dijkstra a de menor custo.
- A ordem do `Plan` respeita todas as arestas `DependsOn` do grafo da API.
- Montar o grafo duas vezes dá o mesmo grafo.

### Hipóteses de invalidade de corretude

É fundamental apresentar as hipóteses de contraposição dos argumentos de corretude anteriores: a
explicação de *por que* cada algoritmo dá a resposta certa, como a fila ordenada da
BFS, "quem termina primeiro" na DFS e "o vértice fechado já está no menor custo"
no Dijkstra. Esses argumentos não valem sempre: cada um depende de condições
específicas. Se a condição for quebrada, não há garantia de que o resultado obtido pela execução da implementação do algoritmo está correto.

**Motivação inicial**

As contraposições de corretude são importantes pois:

- Permitem a inferência da corretude de um resultado;
- Evita um erro silencioso: sem a lista de hipóteses, alguém poderia mudar o
  grafo (por exemplo, pôr um peso negativo) e continuar achando que o resultado
  é o menor caminho.
- Separa o que foi provado do que é suposição, que é o caso da expansão
  trazer o que é relevante.
- Mostra onde os testes precisam "olhar": cada hipótese é um caso de teste.

**Hipóteses**

| Hipótese | Algoritmo | Se for violada |
|---|---|---|
| Pesos maiores ou iguais a 0 | Dijkstra | O caminho pode não ser o de menor custo. Por isso `AddEdge` recusa pesos negativos |
| Sem ciclo em `DependsOn` | `Plan` e ordenação topológica | Não existe ordem válida; o código devolve erro com o ciclo |
| Grafo não muda durante a busca | DFS, BFS e Dijkstra | os invariantes (fila ordenada, custos fechados) deixam de valer. Depois do `Build` o grafo só é lido, então é seguro usar em paralelo |
| Pesos "redondos" (1 e 0,5) | Dijkstra | o computador representa 1 e 0,5 sem erro. Com pesos como 0,1 pode haver erros de arredondamento que mudam um desempate |
| A BFS só vale para "menos arestas" | BFS | se os pesos forem diferentes, ela **não** dá o caminho mais barato; use Dijkstra |
| As provas valem para as arestas que o filtro deixa passar | DFS, BFS e Dijkstra | `Plan` usa só `DependsOn`; a expansão usa as demais. Misturar muda o resultado |
| "Perto" quer dizer "relevante" | expansão | sem garantia. Pode trazer operações próximas, mas inúteis |
| `DependsOn` deduzida do path está certa | `Plan` | em casos como `{owner}`/`{repo}` (que não vêm de uma lista), ou quando o pai não tem `GET`, a dependência é errada ou falta |
| Corpus real parecido com o de teste | DFS, BFS e Dijkstra | Não medi no corpus real; paths muito diferentes ou muitas colisões de método + path podem mudar a estrutura e o desempenho |