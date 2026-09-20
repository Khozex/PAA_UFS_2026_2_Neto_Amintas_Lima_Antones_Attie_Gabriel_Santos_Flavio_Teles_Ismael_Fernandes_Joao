# PAA UFS 2026.2 — Atividade 1

Protótipo de recuperação de contexto sobre a documentação da GitHub REST API
(tema 7: documentação de APIs públicas). Recebe uma pergunta em linguagem
natural e devolve as k operações da API mais relevantes, em ordem, com texto
e URL da documentação para compor o contexto de uma aplicação RAG.

Pergunta de recuperação: "qual endpoint cria (lista, apaga, ...) determinado recurso?"

## Ambiente

- Go 1.26 ou superior, sem dependências externas.
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
  main.go              CLI: flags, carga, índice, uma consulta ou o gabarito inteiro
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

## Testes

```
go test ./...
```

Cobrem normalização, comparador (ordem total), os quatro algoritmos de
ordenação contra a referência estável da biblioteca, seleção com k=0, k=1
e k>N, score determinístico, igualdade entre busca linear e indexada para
todas as combinações de seleção e algoritmo, e as métricas de avaliação.

## Licença

Código sob MIT (`LICENSE`). O corpus é a especificação pública da GitHub
REST API, também MIT.

## Vídeo da atividade

(a preencher: URL, também em `VIDEO.md`)
