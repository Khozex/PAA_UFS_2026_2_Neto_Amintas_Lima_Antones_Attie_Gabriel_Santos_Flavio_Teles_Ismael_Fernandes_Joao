// Espelha ui/mcp (Output/Result) e ui/bridge (Event/Stats).

export type Operation = {
  rank: number
  id: string
  method: string
  path: string
  summary: string
  text: string
  docs_url: string
  score: number
}

export type SearchStats = {
  n: number
  candidates: number
  comparisons: number
  query_us: number
  index_us: number
  config: string
  selection: string
  k: number
}

// Operação vizinha dos resultados no grafo; não vem do ranking, então não tem score.
export type RelatedOp = {
  id: string
  method: string
  path: string
  summary: string
  docs_url: string
  hops: number
}

export type RelatedStats = {
  hops: number
  visited: number
  edges_scanned: number
  expand_us: number
  graph_us: number
}

export type PlanStep = { id: string; method: string; path: string; summary: string }

export type CallPlan = { rank: number; steps: PlanStep[] }

export type PlanStats = {
  with_prerequisites: number
  prerequisites: number
  edges_scanned: number
  plan_us: number
  graph_us: number
}

export type SearchOutput = {
  query: string
  results: Operation[]
  stats: SearchStats
  related?: RelatedOp[]
  related_stats?: RelatedStats
  plans?: CallPlan[]
  plan_stats?: PlanStats
}

// O que a tela pode mudar por pergunta; vai como argumentos da tool MCP.
export type SearchOptions = {
  k: number
  config: 'linear' | 'indexed'
  select: 'topk' | 'heap' | 'sort'
  order_strategy: 'quick' | 'heap' | 'merge' | 'std'
  plan: boolean
  related: number
  hops: number
}

type GraphOptions = 'plan' | 'related' | 'hops'

export const presets: { id: string; label: string; hint: string; opts: Omit<SearchOptions, 'k' | GraphOptions> }[] = [
  { id: '1', label: 'Preset 1 · varredura linear + top-k', hint: 'Varre os 1224 documentos: ~610 µs por consulta', opts: { config: 'linear', select: 'topk', order_strategy: 'merge' } },
  { id: '2', label: 'Preset 2 · índice invertido + top-k', hint: 'O mais rápido no benchmark: ~375 µs por consulta', opts: { config: 'indexed', select: 'topk', order_strategy: 'merge' } },
  { id: '3', label: 'Preset 3 · varredura linear + merge sort', hint: 'Ordena todos os candidatos e corta em k: ~670 µs por consulta', opts: { config: 'linear', select: 'sort', order_strategy: 'merge' } },
]

export const defaultSearch: SearchOptions = { k: 3, ...presets[1].opts, plan: false, related: 0, hops: 3 }

export type Stats = {
  rounds: number
  tool_calls: number
  prompt_tokens: number
  output_tokens: number
  llm_ms: number
}

export type BridgeEvent =
  | { type: 'token'; text: string }
  | { type: 'tool_call'; name: string; args: Record<string, unknown> }
  | { type: 'tool_result'; name: string; text: string; data?: SearchOutput }
  | { type: 'done'; answer: string; stats: Stats }
  | { type: 'error'; text: string }

export type ToolTrace = {
  name: string
  args: Record<string, unknown>
  result?: SearchOutput
  resultText?: string
}

export type Turn = {
  id: number
  question: string
  answer: string
  tools: ToolTrace[]
  search: SearchOptions
  stats?: Stats
  error?: string
  pending: boolean
}

export type ModelInfo = { name: string; size: number; tools: boolean }
