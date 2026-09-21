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

export type SearchOutput = {
  query: string
  results: Operation[]
  stats: SearchStats
}

// O que a tela pode mudar por pergunta; vai como argumentos da tool MCP.
export type SearchOptions = {
  k: number
  config: 'linear' | 'indexed'
  select: 'topk' | 'heap' | 'sort'
  order_strategy: 'quick' | 'heap' | 'merge' | 'std'
}

export const presets: { id: string; label: string; hint: string; opts: Omit<SearchOptions, 'k'> }[] = [
  { id: '2', label: 'Preset 2 · indexado + topk', hint: 'o mais rápido no benchmark: ~375 µs por consulta', opts: { config: 'indexed', select: 'topk', order_strategy: 'merge' } },
  { id: '1', label: 'Preset 1 · linear + topk', hint: 'varre os 1224 documentos: ~610 µs', opts: { config: 'linear', select: 'topk', order_strategy: 'merge' } },
  { id: '3', label: 'Preset 3 · linear + sort/merge', hint: 'ordena todos os candidatos e corta k: ~670 µs', opts: { config: 'linear', select: 'sort', order_strategy: 'merge' } },
]

export const defaultSearch: SearchOptions = { k: 3, ...presets[0].opts }

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
  stats?: Stats
  error?: string
  pending: boolean
}

export type ModelInfo = { name: string; size: number; tools: boolean }
