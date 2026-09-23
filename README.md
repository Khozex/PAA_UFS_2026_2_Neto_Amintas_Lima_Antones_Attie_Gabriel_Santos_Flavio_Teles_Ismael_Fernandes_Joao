# PAA UFS 2026.2 — Atividade 1

Protótipo de recuperação de contexto sobre a documentação da GitHub REST API
(tema 7: documentação de APIs públicas). Recebe uma pergunta em linguagem
natural e devolve as k operações da API mais relevantes, em ordem, com texto
e URL da documentação para compor o contexto de uma aplicação RAG.

Pergunta de recuperação: "qual endpoint cria (lista, apaga, ...) determinado recurso?"

## Ambiente

- Go 1.26 ou superior. O núcleo (ingestão, busca, benchmark) não tem
  dependências externas; só `ui/` usa o SDK oficial de MCP em Go
  (`github.com/modelcontextprotocol/go-sdk`).
- Para o chat com modelo local (`ui/chat`): [Ollama](https://ollama.com)
  rodando com um modelo que suporte tools, por exemplo `ollama pull qwen2.5:3b`.
- Testado em Linux x86_64 (Fedora, AMD Ryzen 7 7800X3D, 30 GiB). O benchmark
  registra o hardware da máquina em que roda.
- Corpus: `data/githubApiDoc.json`, descrição OpenAPI 3.0.3 da GitHub REST
  API v1.1.4, licença MIT, obtida de
  https://github.com/github/rest-api-description
  (`descriptions/api.github.com/api.github.com.json`). Ficha completa em
  `artefatos/ficha-do-corpus.pdf`.

Todos os comandos rodam de qualquer pasta do repositório: os caminhos são
resolvidos a partir do `go.mod`.

## Reproduzir do zero

```
go run ./ingestDataPipeline      # 1. gera data/processed/docs.jsonl e manifest.json
go test ./...                    # 2. testes de normalização, comparador, ordenação, seleção, score, busca e avaliação
go run ./searchEngine -eval      # 3. Precision@k das 10 perguntas do gabarito
go run ./benchmark               # 4. 27 cenários × 10 perguntas × 2 repetições → artefatos/benchmark_results.{txt,csv}
go run ./ui/server               # 5. (opcional) interface web RAG com modelo local via MCP; precisa do Ollama e do build em ui/web
```

## Layout

```
ingestDataPipeline/    OpenAPI JSON → docs.jsonl (uma operação por documento)
  main.go              load → extract → clean → write, cronometra cada etapa e grava manifest.json
  types/               structs do OpenAPI, Document, Field, Manifest
  utils/               LoadSpec, WriteJSONL, WriteManifest, Resolve
  extract/             um Document por par método+path; resolve $ref um nível
  clean/               remove boilerplate, markdown e HTML; monta o campo text
  stats/               total e mín/mediana/média/máx de palavras por documento

searchEngine/          lê docs.jsonl e responde consultas
  main.go              CLI: flags, uma consulta ou o gabarito inteiro
  engine/              Options, Load (corpus + índice) e Query/QueryK; é o que o CLI
                       e o servidor MCP usam
  types/               Document, Corpus, Hit, Stats, Query
  utils/               ReadJSONL, ReadQueries, Normalize (lowercase, tokens, stopwords),
                       Resolve, MoreRelevant (comparador único: score → tamanho → id)
  corpus/              Load: TF por campo, DF e IDF de cada termo
  ranking/             Score: TF-IDF com tf sublinear e pesos por campo
                       (summary 3, path 2, description 1, params 1)
  linearSearch/        Configuração 1: varre todos os documentos
  indexedSearch/       Configuração 2: índice invertido termo → documentos;
                       pontua só os candidatos que contêm algum termo da consulta
  selection/           como escolher os k melhores entre os candidatos pontuados:
                         topk  inserção ordenada durante a varredura, Θ(n·k)
                         heap  min-heap de tamanho k, Θ(n log k)
                         sort  ordena todos os candidatos e corta k, Θ(n log n)  (Configuração 3)
  sorting/             algoritmos usados por -select sort:
                         quick  pivô por mediana de três, in-place, instável
                         heap   heap sort, in-place, instável
                         merge  merge sort, buffer de n, estável
                         std    slices.SortFunc da biblioteca padrão (baseline, não é da equipe)
  eval/                Rank, Precision@k e resumo sobre o gabarito

ui/                    o searchEngine como tool de um modelo de linguagem (RAG)
  mcp/                 servidor MCP por stdio com a tool search_github_api
  bridge/              ponte Ollama ↔ MCP: reescrita da pergunta, busca obrigatória, resposta
  server/              HTTP + SSE que serve a app React
  web/                 app React (Vite)
  chat/                a ponte no terminal, para testes

benchmark/             compila o searchEngine uma vez e roda a matriz de cenários
  main.go              laço cenário × pergunta × repetição, grava txt e csv
  scenarios/           presets 1, 2, 3 × corpus completo e metade; seleção × k ∈ {5, 50, 500, 1224}
  report/              extrai a linha de estatísticas e os resultados; calcula posição e P@k
  hardware/            CPU, núcleos e RAM da máquina

data/
  githubApiDoc.json    corpus bruto (13 MB)
  queries.json         gabarito: 10 perguntas → operationId esperado
  processed/           gerado pela ingestão, fora do git

artefatos/
  ficha-do-corpus.pdf
  benchmark_results.txt  log de cada execução (5 primeiros resultados + estatísticas)
  benchmark_results.csv  uma linha por execução
```

## Fluxo de uma consulta

```
pergunta → Normalize → busca (linear ou indexada) pontua candidatos com Score
         → seleção dos k (topk | heap | sort) usando MoreRelevant
         → k resultados + estatísticas (+ texto e docs_url com -context)
```

Ordenação aparece uma vez só: na seleção dos k. As três configurações usam
o mesmo score e o mesmo comparador, então devolvem a mesma lista para a
mesma pergunta; o que muda entre elas é o custo.

## Buscar

```
go run ./searchEngine -query "create a repository in an organization"
go run ./searchEngine -query "cancel a workflow run" -k 3 -context
go run ./searchEngine -query "..." -preset 2
```

| flag | padrão | o que faz |
|---|---|---|
| `-query` | create a repository in an organization | pergunta |
| `-k` | 5 | quantos resultados |
| `-preset` | 0 | atalho: 1 = linear+topk, 2 = indexed+topk, 3 = linear+sort/merge |
| `-config` | linear | `linear` ou `indexed` |
| `-select` | topk | `topk`, `heap` ou `sort` |
| `-order_strategy` | merge | `quick`, `heap`, `merge` ou `std`; só tem efeito com `-select sort` |
| `-order` | desc | `asc` inverte a lista final |
| `-limit` | 0 | usa só os N primeiros documentos (tamanho de corpus) |
| `-context` | false | imprime texto e docs_url de cada resultado |
| `-eval` | false | roda todas as perguntas do gabarito e imprime P@k |
| `-queries` | data/queries.json | caminho do gabarito |
| `-corpus` | data/processed/docs.jsonl | caminho do corpus processado |

Última linha de saída de uma consulta:

```
N=1224  candidatos=94  comparações=126  seleção=topk  carga=59ms  índice=0s  mem=6684kB  consulta=396µs
```

`N` documentos no corpus, `candidatos` com score > 0, `comparações` feitas
pela seleção, tempo de carga, de construção do índice (0 no linear),
memória de heap após carga e índice, tempo da consulta.

## Avaliar

```
go run ./searchEngine -eval
go run ./searchEngine -eval -preset 2 -k 10
```

Imprime, por pergunta, candidatos, comparações, tempo, posição da resposta
esperada (0 = fora dos k) e P@k; no fim, acertos@k, vazias, P@k médio,
posição média e tempos. Com uma resposta certa por pergunta o teto de P@k
é 1/k; acertos@k e posição média são as métricas mais informativas.

## Benchmark

```
go run ./benchmark
```

Gera `artefatos/benchmark_results.csv` com as colunas
`cenario, preset, config, selecao, k, N, query, repeticao, candidatos,
comparacoes, carga_ms, indice_us, mem_kb, consulta_us, posicao, p_at_k`
e o log correspondente em `.txt`, com SO, Go, CPU, núcleos e RAM no
cabeçalho. Cenários:

- presets 1, 2 e 3 com corpus completo (1224) e metade (612), k=5;
- topk, heap, sort/merge, sort/quick e sort/std com k ∈ {5, 50, 500, 1224}.

## Chat com modelo local (RAG via MCP)

O `searchEngine` vira uma tool do
[Model Context Protocol](https://modelcontextprotocol.io) e um modelo local
(Ollama) responde perguntas sobre a API usando só o que a busca recuperou.
É a aplicação RAG para a qual o motor de busca foi feito.

Cada pergunta passa por três etapas fixas, sem depender de o modelo decidir
chamar a tool:

1. o modelo reescreve a pergunta como uma consulta curta em inglês
   ("como apago uma release?" → `delete release`), usando o histórico para
   resolver referências;
2. a ponte chama `search_github_api` via MCP e recebe as k operações com o
   texto completo (o mesmo que `-context` no CLI);
3. o modelo responde com esses trechos: endpoint, parâmetros, exemplo curl e
   link da documentação, no idioma da pergunta.

```
ui/
  mcp/      servidor MCP por stdio com a tool search_github_api
  bridge/   ponte Ollama ↔ MCP: reescrita, busca obrigatória, resposta em streaming
  server/   HTTP: /api/models, /api/chat (SSE) e a app React
  web/      app React (Vite): conversa à esquerda, recuperação à direita
  chat/     a mesma ponte no terminal, para testes
```

### Rodar

```
ollama pull qwen2.5:3b                 # uma vez; precisa ser um modelo com tools
cd ui/web && npm install && npm run build && cd ../..
go run ./ui/server                     # abre http://localhost:8080
```

Para mexer no React com hot reload: `cd ui/web && npm run dev` em outro
terminal (sobe em :5173 e repassa `/api` para :8080).

| flag de `ui/server` | padrão | o que faz |
|---|---|---|
| `-addr` | :8080 | endereço HTTP |
| `-ollama` | http://localhost:11434 | endereço do Ollama |
| `-k` | 5 | k padrão quando a tela não manda um |
| `-num_ctx` | 8192 | janela de contexto do modelo |
| `-static` | ui/web/dist | build da app React |
| `-mcp` | | binário do servidor MCP; vazio usa `go run ./ui/mcp` |

Flags após `--` vão para o servidor MCP, ex.: `go run ./ui/server -- -preset 3`.
O modelo é escolhido na própria interface, entre os instalados no Ollama
que aceitam tools (gemma3 não aceita).

A interface mostra, para cada pergunta, a consulta que o modelo montou, os
endpoints ranqueados que a busca devolveu (com score, texto e link), o custo
da busca (N, candidatos, comparações, tempo, índice) e o custo do modelo
(chamadas, tokens, tempo). A busca leva microssegundos; o tempo é todo do
modelo (qwen2.5:3b na CPU: 30 a 60 s por resposta).

A busca é configurável na própria tela, por pergunta, sem reiniciar nada:
`k` (1 a 10) e a configuração, como preset 1, 2 ou 3 ou personalizada
(config linear/indexed, seleção topk/heap/sort e algoritmo de ordenação).
Os valores vão como argumentos da tool MCP. O padrão é k=3 com o preset 2
(indexado + topk), o mais rápido no benchmark. Sobre k: no gabarito a
resposta certa está em 1º lugar em 6 de 10 perguntas e até o 3º em 9 de 10,
então k=1 perderia respostas que k=3 acerta.

### Só o servidor MCP

O servidor MCP também roda sozinho e serve a qualquer cliente MCP
(MCP Inspector, ollmcp, Claude Desktop, editores):

```
go run ./ui/mcp [-preset 2] [-max_chars 0] [-config ...] [-select ...]
```

Uma tool só, `search_github_api(query, k, config, select, order_strategy)`;
só `query` é obrigatório. `k` vai de 1 a 10 (padrão 3); `config`, `select` e
`order_strategy` mudam a busca de uma chamada e, se vazios, usam o que o
servidor recebeu por flag. O resultado traz as operações e as estatísticas
da consulta (N, candidatos, comparações, tempo, índice). `-max_chars` corta
o texto de cada operação, para clientes com contexto curto; o padrão 0
devolve o texto completo.

### Só o terminal

```
go run ./ui/chat -once "how do I cancel a workflow run?" -v
go run ./ui/chat -model llama3.2:1b
```

## Testes

```
go test ./...
```

Cobrem normalização, comparador (ordem total), os quatro algoritmos de
ordenação contra a referência estável da biblioteca, seleção com k=0, k=1
e k>N, score determinístico, igualdade entre busca linear e indexada para
todas as combinações de seleção e algoritmo, as métricas de avaliação e o
`engine` (validação das opções, presets, linear = indexado, QueryK).

## Licença

Código sob MIT (`LICENSE`). O corpus é a especificação pública da GitHub
REST API, também MIT.

## Vídeo da atividade

https://youtu.be/gSicRRvsFK8 (também em `VIDEO.md`)
