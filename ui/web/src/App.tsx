import { useEffect, useRef, useState, type FormEvent } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { fetchModels, streamChat, type ChatMessage } from './api'
import { defaultSearch, presets, type ModelInfo, type Operation, type SearchOptions, type SearchStats, type Turn } from './types'

const suggestions = [
  'how do I cancel a workflow run?',
  'como eu crio um repositório dentro de uma organização?',
  'list the issues of a repository',
  'como eu apago uma release?',
]

const storageKey = 'paa.search'

function loadSearch(): SearchOptions {
  try {
    const raw = localStorage.getItem(storageKey)
    if (raw) return { ...defaultSearch, ...JSON.parse(raw) }
  } catch {
    /* sem storage: usa o padrão */
  }
  return defaultSearch
}

export default function App() {
  const [models, setModels] = useState<ModelInfo[]>([])
  const [model, setModel] = useState('')
  const [modelsError, setModelsError] = useState('')
  const [search, setSearch] = useState<SearchOptions>(loadSearch)
  const [turns, setTurns] = useState<Turn[]>([])
  const [selected, setSelected] = useState<number | null>(null)
  const [draft, setDraft] = useState('')
  const abort = useRef<AbortController | null>(null)
  const nextId = useRef(1)

  useEffect(() => {
    fetchModels()
      .then((list) => {
        setModels(list)
        const first = list.find((m) => m.tools) ?? list[0]
        if (first) setModel(first.name)
      })
      .catch((err: Error) => setModelsError(err.message))
  }, [])

  useEffect(() => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(search))
    } catch {
      /* sem storage: só não lembra */
    }
  }, [search])

  const busy = turns.some((t) => t.pending)
  const current = turns.find((t) => t.id === selected) ?? turns[turns.length - 1]

  function patch(id: number, fn: (t: Turn) => Turn) {
    setTurns((prev) => prev.map((t) => (t.id === id ? fn(t) : t)))
  }

  async function ask(question: string) {
    const text = question.trim()
    if (!text || busy || !model) return
    const id = nextId.current++
    const history: ChatMessage[] = turns.flatMap((t) => [
      { role: 'user', content: t.question },
      { role: 'assistant', content: t.answer },
    ])
    history.push({ role: 'user', content: text })

    setTurns((prev) => [...prev, { id, question: text, answer: '', tools: [], pending: true }])
    setSelected(id)
    setDraft('')
    abort.current = new AbortController()

    try {
      await streamChat(
        model,
        history,
        search,
        (e) => {
          switch (e.type) {
            case 'token':
              patch(id, (t) => ({ ...t, answer: t.answer + e.text }))
              break
            case 'tool_call':
              patch(id, (t) => ({ ...t, tools: [...t.tools, { name: e.name, args: e.args }] }))
              break
            case 'tool_result':
              patch(id, (t) => {
                const tools = [...t.tools]
                const last = tools.length - 1
                if (last >= 0) tools[last] = { ...tools[last], result: e.data, resultText: e.text }
                return { ...t, tools }
              })
              break
            case 'done':
              patch(id, (t) => ({ ...t, answer: e.answer, stats: e.stats, pending: false }))
              break
            case 'error':
              patch(id, (t) => ({ ...t, error: e.text, pending: false }))
              break
          }
        },
        abort.current.signal,
      )
    } catch (err) {
      patch(id, (t) => ({ ...t, error: (err as Error).message, pending: false }))
    } finally {
      patch(id, (t) => ({ ...t, pending: false }))
    }
  }

  function stop() {
    abort.current?.abort()
  }

  function onSubmit(ev: FormEvent) {
    ev.preventDefault()
    ask(draft)
  }

  return (
    <div className="app">
      <header className="top">
        <div>
          <h1>GitHub REST API</h1>
          <p className="sub">Pergunte qual endpoint faz o quê. O modelo reescreve a pergunta como consulta, o motor de busca do projeto recupera as operações, e a resposta usa só o que foi recuperado.</p>
        </div>
        <label className="model">
          <span>Modelo</span>
          <select value={model} onChange={(e) => setModel(e.target.value)} disabled={busy || models.length === 0}>
            {models.map((m) => (
              <option key={m.name} value={m.name} disabled={!m.tools}>
                {m.name} · {gb(m.size)}{m.tools ? '' : ' · sem tools'}
              </option>
            ))}
          </select>
          {modelsError && <em className="err">Ollama não respondeu: {modelsError}</em>}
        </label>
      </header>

      <main className="cols">
        <section className="chat" aria-label="Conversa">
          {turns.length === 0 && (
            <div className="empty">
              <p>Nada perguntado ainda. Experimente:</p>
              <ul>
                {suggestions.map((s) => (
                  <li key={s}>
                    <button type="button" onClick={() => ask(s)} disabled={busy || !model}>
                      {s}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {turns.map((t) => (
            <article key={t.id} className={'turn' + (t === current ? ' is-selected' : '')} onClick={() => setSelected(t.id)}>
              <p className="q">{t.question}</p>
              <TraceLine turn={t} />
              {t.error ? (
                <p className="err">Falhou: {t.error}</p>
              ) : t.answer ? (
                <Answer text={t.answer} streaming={t.pending} />
              ) : t.pending ? (
                <p className="thinking">{t.tools.length === 0 ? 'Montando a consulta…' : 'Lendo a documentação recuperada…'}</p>
              ) : null}
            </article>
          ))}

          <form className="composer" onSubmit={onSubmit}>
            <input
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder={model ? 'Qual endpoint…' : 'Aguardando o Ollama'}
              disabled={busy || !model}
              autoFocus
            />
            {busy ? (
              <button type="button" onClick={stop}>Parar</button>
            ) : (
              <button type="submit" disabled={!draft.trim() || !model}>Perguntar</button>
            )}
          </form>
        </section>

        <aside className="side" aria-label="Busca">
          <SearchSettings value={search} onChange={setSearch} disabled={busy} />
          <div className="retrieval">
            {current ? <Retrieval turn={current} /> : (
              <p className="hint">Aqui aparece o que o motor de busca devolveu para o modelo: a consulta que ele montou, os endpoints ranqueados e o custo.</p>
            )}
          </div>
        </aside>
      </main>
    </div>
  )
}

function SearchSettings({ value, onChange, disabled }: { value: SearchOptions; onChange: (v: SearchOptions) => void; disabled: boolean }) {
  const presetId = presets.find((p) => sameOpts(p.opts, value))?.id ?? 'custom'
  const [custom, setCustom] = useState(presetId === 'custom')
  const preset = presets.find((p) => p.id === presetId)

  function pickPreset(id: string) {
    if (id === 'custom') {
      setCustom(true)
      return
    }
    setCustom(false)
    const p = presets.find((x) => x.id === id)!
    onChange({ ...value, ...p.opts })
  }

  return (
    <form className="settings" onSubmit={(e) => e.preventDefault()}>
      <div className="settings-row">
        <label>
          <span>Resultados (k)</span>
          <input
            type="number" min={1} max={10} value={value.k} disabled={disabled}
            onChange={(e) => onChange({ ...value, k: clamp(Number(e.target.value), 1, 10) })}
          />
        </label>
        <label className="grow">
          <span>Configuração da busca</span>
          <select value={custom ? 'custom' : presetId} disabled={disabled} onChange={(e) => pickPreset(e.target.value)}>
            {presets.map((p) => <option key={p.id} value={p.id}>{p.label}</option>)}
            <option value="custom">Personalizada</option>
          </select>
        </label>
      </div>
      {custom || presetId === 'custom' ? (
        <div className="settings-row">
          <label>
            <span>Config</span>
            <select value={value.config} disabled={disabled} onChange={(e) => onChange({ ...value, config: e.target.value as SearchOptions['config'] })}>
              <option value="indexed">indexed</option>
              <option value="linear">linear</option>
            </select>
          </label>
          <label>
            <span>Seleção</span>
            <select value={value.select} disabled={disabled} onChange={(e) => onChange({ ...value, select: e.target.value as SearchOptions['select'] })}>
              <option value="topk">topk</option>
              <option value="heap">heap</option>
              <option value="sort">sort</option>
            </select>
          </label>
          <label>
            <span>Algoritmo</span>
            <select value={value.order_strategy} disabled={disabled || value.select !== 'sort'} onChange={(e) => onChange({ ...value, order_strategy: e.target.value as SearchOptions['order_strategy'] })}>
              <option value="merge">merge</option>
              <option value="quick">quick</option>
              <option value="heap">heap</option>
              <option value="std">std</option>
            </select>
          </label>
        </div>
      ) : (
        <p className="settings-hint">{preset?.hint}. k={value.k}: no gabarito a resposta certa está em 1º em 6 de 10 perguntas e até o 3º em 9 de 10.</p>
      )}
    </form>
  )
}

function sameOpts(a: Omit<SearchOptions, 'k'>, b: SearchOptions) {
  return a.config === b.config && a.select === b.select && (a.select !== 'sort' || a.order_strategy === b.order_strategy)
}

function TraceLine({ turn }: { turn: Turn }) {
  if (turn.tools.length === 0) return null
  return (
    <ul className="trace">
      {turn.tools.map((tool, i) => (
        <li key={i}>
          <code>{String(tool.args.query ?? '')}</code>
          <span>{tool.result ? `${tool.result.results.length} endpoints em ${us(tool.result.stats.query_us)}` : 'buscando…'}</span>
        </li>
      ))}
    </ul>
  )
}

function Retrieval({ turn }: { turn: Turn }) {
  const tool = turn.tools[turn.tools.length - 1]
  return (
    <div className="retrieval-body">
      <div className="retrieval-head">
        <h2>O que a busca devolveu</h2>
        {tool ? (
          <p>
            consulta do modelo: <code>{String(tool.args.query ?? '')}</code>
          </p>
        ) : (
          <p>{turn.pending ? 'O modelo está reescrevendo a pergunta como consulta…' : 'A busca não foi executada.'}</p>
        )}
      </div>

      {tool?.result && <Results ops={tool.result.results} />}
      {tool && !tool.result && <p className="hint">buscando…</p>}

      {tool?.result && <SearchCost stats={tool.result.stats} />}

      {turn.stats && (
        <dl className="stats">
          <div><dt>chamadas ao modelo</dt><dd>{turn.stats.rounds}</dd></div>
          <div><dt>tokens de entrada</dt><dd>{turn.stats.prompt_tokens}</dd></div>
          <div><dt>tokens gerados</dt><dd>{turn.stats.output_tokens}</dd></div>
          <div><dt>tempo no modelo</dt><dd>{(turn.stats.llm_ms / 1000).toFixed(1)} s</dd></div>
        </dl>
      )}
    </div>
  )
}

function SearchCost({ stats }: { stats: SearchStats }) {
  return (
    <dl className="stats cost">
      <div><dt>busca</dt><dd>{stats.config} · {stats.selection}</dd></div>
      <div><dt>documentos (N)</dt><dd>{stats.n}</dd></div>
      <div><dt>candidatos</dt><dd>{stats.candidates}</dd></div>
      <div><dt>comparações</dt><dd>{stats.comparisons}</dd></div>
      <div><dt>tempo da consulta</dt><dd>{us(stats.query_us)}</dd></div>
      <div><dt>índice</dt><dd>{stats.config === 'indexed' ? us(stats.index_us) + ' (uma vez)' : 'não usa'}</dd></div>
    </dl>
  )
}

function Results({ ops }: { ops: Operation[] }) {
  const [open, setOpen] = useState<number | null>(null)
  if (ops.length === 0) return <p className="hint">Nenhum endpoint com score maior que zero.</p>
  const top = ops[0].score || 1
  return (
    <ol className="ops">
      {ops.map((op) => (
        <li key={op.id} className={open === op.rank ? 'is-open' : ''}>
          <button type="button" onClick={() => setOpen(open === op.rank ? null : op.rank)} aria-expanded={open === op.rank}>
            <span className={'method m-' + op.method.toLowerCase()}>{op.method}</span>
            <code className="path">{op.path}</code>
            <span className="summary">{op.summary}</span>
            <span className="score" style={{ ['--w' as string]: `${(100 * op.score) / top}%` }}>
              <i /> {op.score.toFixed(1)}
            </span>
          </button>
          {open === op.rank && (
            <div className="op-text">
              <p>{op.text}</p>
              <a href={op.docs_url} target="_blank" rel="noreferrer">Documentação de {op.id}</a>
            </div>
          )}
        </li>
      ))}
    </ol>
  )
}

// A resposta do modelo vem em Markdown (listas, blocos de código, links).
// react-markdown não interpreta HTML cru, então o texto do modelo é seguro.
function Answer({ text, streaming }: { text: string; streaming: boolean }) {
  return (
    <div className={'answer' + (streaming ? ' is-streaming' : '')}>
      <Markdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href, children }) => <a href={href} target="_blank" rel="noreferrer">{children}</a>,
        }}
      >
        {text}
      </Markdown>
    </div>
  )
}

function us(n: number) {
  return n >= 1000 ? (n / 1000).toFixed(1) + ' ms' : n + ' µs'
}

function clamp(n: number, lo: number, hi: number) {
  return Number.isFinite(n) ? Math.min(hi, Math.max(lo, n)) : lo
}

function gb(bytes: number) {
  return (bytes / 1e9).toFixed(1) + ' GB'
}
