import type { BridgeEvent, ModelInfo, SearchOptions } from './types'

export async function fetchModels(): Promise<ModelInfo[]> {
  const res = await fetch('/api/models')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export type ChatMessage = { role: 'user' | 'assistant'; content: string }

// POST /api/chat responde em SSE; EventSource só faz GET, então lemos o corpo à mão.
export async function streamChat(
  model: string,
  messages: ChatMessage[],
  search: SearchOptions,
  onEvent: (e: BridgeEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch('/api/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ model, messages, search }),
    signal,
  })
  if (!res.ok || !res.body) throw new Error(await res.text())

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let sep: number
    while ((sep = buffer.indexOf('\n\n')) >= 0) {
      const frame = buffer.slice(0, sep)
      buffer = buffer.slice(sep + 2)
      for (const line of frame.split('\n')) {
        if (line.startsWith('data: ')) onEvent(JSON.parse(line.slice(6)))
      }
    }
  }
}
