# PAA UFS 2026.2 — Atividade 1

Recuperação de contexto sobre a documentação da GitHub REST API (tema 7).

## Layout

```
ingestDataPipeline/       lê data/githubApiDoc.json, grava data/processed/docs.jsonl + manifest.json
  main.go                 execução: load → extract → clean → write, com tempos
  types/                  structs: Spec (OpenAPI), Document, Field, Manifest
  utils/                  LoadSpec, WriteJSONL, WriteManifest
  extract/                Extract: um Document por operação, resolve $ref
  clean/                  Clean: regexes de limpeza, monta o campo text
  stats/                  Words: total e mín/mediana/média/máx de palavras por documento
searchEngine/             lê docs.jsonl e responde consultas
  main.go                 execução
  types/                  structs: Document, Field
  utils/                  ReadJSONL
```

## Rodar

Realizar ingestão dos dados:
```
go run ./ingestDataPipeline
```

Buscas:
```
go run ./searchEngine -query "create a repository in an organization" -k 5
go run ./searchEngine -query "..." -limit 300        # subconjunto do corpus
go run ./searchEngine -query "..." -context          # imprime text e docs_url dos resultados
```

Uso de ordenação:
```
go run ./searchEngine -query "create a repository in an organization" -k 5 -order desc -order_strategy heap
```

## Stack

Go >=1.26 sem dependências externas.

## Vídeo da atividade

(a preencher)
