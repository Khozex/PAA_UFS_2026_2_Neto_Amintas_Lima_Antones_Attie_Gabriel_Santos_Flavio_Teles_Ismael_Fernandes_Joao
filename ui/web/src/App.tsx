import { useEffect, useRef, useState, type FormEvent } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { fetchModels, streamChat, type ChatMessage } from './api'
import { buildRows, collectOps, type TreeOp } from './pathTree'
import { defaultSearch, presets, type CallPlan, type ModelInfo, type SearchOptions, type SearchOutput, type Turn } from './types'

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
  const [settingsOpen, setSettingsOpen] = useState(false)
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

    setTurns((prev) => [...prev, { id, question: text, answer: '', tools: [], search, pending: true }])
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
          <h1>Busca na API REST do GitHub</h1>
          <p className="sub">Pergunte o que quer fazer. A resposta usa só as operações que a busca encontrou.</p>
        </div>
        <label className="model">
          <span>Modelo</span>
          <select value={model} onChange={(e) => setModel(e.target.value)} disabled={busy || models.length === 0}>
            {models.map((m) => (
              <option key={m.name} value={m.name} disabled={!m.tools}>
                {m.name} ({gb(m.size)}{m.tools ? '' : ', sem suporte a tools'})
              </option>
            ))}
          </select>
          {modelsError && <em className="err">O Ollama não respondeu: {modelsError}</em>}
        </label>
      </header>

      <div className="settings-bar">
        <p><strong>Busca</strong> {searchSummary(search)}</p>
        <p><strong className="graph-word">Grafo</strong> {graphSummary(search)}</p>
        <button type="button" className="secondary" onClick={() => setSettingsOpen(!settingsOpen)} aria-expanded={settingsOpen}>
          {settingsOpen ? 'Fechar ajustes' : 'Ajustar busca'}
        </button>
      </div>
      {settingsOpen && <SearchSettings value={search} onChange={setSearch} disabled={busy} />}

      <main className="cols">
        <section className="panel chat" aria-label="Conversa">
          {turns.length === 0 && (
            <div className="empty">
              <p>Nada perguntado ainda. Experimente uma destas:</p>
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
              {t.error ? (
                <p className="err">A pergunta falhou: {t.error}</p>
              ) : t.answer ? (
                <Answer text={t.answer} streaming={t.pending} />
              ) : t.pending ? (
                <p className="thinking">{t.tools.length === 0 ? 'Transformando a pergunta em busca…' : 'Lendo as operações encontradas…'}</p>
              ) : null}
            </article>
          ))}

          <form className="composer" onSubmit={onSubmit}>
            <label className="grow">
              <span className="visually-hidden">Pergunta</span>
              <input
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                placeholder={model ? 'O que você quer fazer na API?' : 'Esperando o Ollama'}
                disabled={busy || !model}
                autoFocus
              />
            </label>
            {busy ? (
              <button type="button" className="primary" onClick={stop}>Parar</button>
            ) : (
              <button type="submit" className="primary" disabled={!draft.trim() || !model}>Perguntar</button>
            )}
          </form>
        </section>

        <section className="panel retrieval" aria-label="Como a resposta foi montada">
          <h2>Como a resposta foi montada</h2>
          {current && (
            <p className="made-with">
              <span><strong>Busca</strong> {searchSummary(current.search)}</span>
              <span><strong className="graph-word">Grafo</strong> {graphSummary(current.search)}</span>
            </p>
          )}
          {current ? <Retrieval turn={current} /> : (
            <p className="hint">Faça uma pergunta para ver aqui o que a busca entregou ao modelo, desenhado na árvore da API.</p>
          )}
        </section>
      </main>
    </div>
  )
}

function searchSummary(s: SearchOptions) {
  const how = s.config === 'indexed' ? 'índice invertido' : 'varredura linear'
  return `${s.k} ${s.k === 1 ? 'resultado' : 'resultados'}, ${how}`
}

function graphSummary(s: SearchOptions) {
  const parts: string[] = []
  if (s.related > 0) parts.push(`${s.related} ${s.related === 1 ? 'vizinha' : 'vizinhas'} a até ${s.hops} passos`)
  if (s.plan) parts.push('ordem de chamadas')
  return parts.length === 0 ? 'desligado' : parts.join(', com ')
}

function SearchSettings({ value, onChange, disabled }: { value: SearchOptions; onChange: (v: SearchOptions) => void; disabled: boolean }) {
  const presetId = presets.find((p) => sameOpts(p.opts, value))?.id ?? 'custom'
  const [custom, setCustom] = useState(presetId === 'custom')
  const preset = presets.find((p) => p.id === presetId)
  const graphOn = value.related > 0 || value.plan

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
          <span>Resultados (k)<Help text="Quantas operações a busca devolve, em ordem de nota." /></span>
          <input
            type="number" min={1} max={10} value={value.k} disabled={disabled}
            onChange={(e) => onChange({ ...value, k: clamp(Number(e.target.value), 1, 10) })}
          />
        </label>
        <Choice
          label="Configuração da busca" value={custom ? 'custom' : presetId} disabled={disabled} onChange={pickPreset}
          help="Como a busca acha e ordena as operações. Muda o tempo, não o resultado."
          options={[...presets.map((p) => ({ value: p.id, label: p.label })), { value: 'custom', label: 'Personalizada' }]}
        />
      </div>
      {custom || presetId === 'custom' ? (
        <div className="settings-row">
          <Choice
            label="Onde procura" value={value.config} disabled={disabled}
            onChange={(config) => onChange({ ...value, config })}
            options={[{ value: 'indexed', label: 'Índice invertido' }, { value: 'linear', label: 'Varredura linear' }]}
          />
          <Choice
            label="Como escolhe os k melhores" value={value.select} disabled={disabled}
            onChange={(select) => onChange({ ...value, select })}
            options={[{ value: 'topk', label: 'Top-k' }, { value: 'heap', label: 'Heap' }, { value: 'sort', label: 'Ordenação completa' }]}
          />
          <Choice
            label="Algoritmo de ordenação" value={value.order_strategy} disabled={disabled || value.select !== 'sort'}
            onChange={(order_strategy) => onChange({ ...value, order_strategy })}
            options={[{ value: 'merge', label: 'Merge sort' }, { value: 'quick', label: 'Quick sort' }, { value: 'heap', label: 'Heap sort' }, { value: 'std', label: 'Sort do Go' }]}
          />
        </div>
      ) : (
        <p className="settings-hint">{preset?.hint}.</p>
      )}
      <div className="settings-row graph-row">
        <Choice
          label="Grafo" value={graphOn ? 'on' : 'off'} disabled={disabled}
          help="Usa o grafo da API para trazer operações além das que a busca achou. Desligado, o modelo só vê os resultados da busca."
          onChange={(v) => onChange(v === 'on' ? { ...value, related: 2 } : { ...value, related: 0, plan: false })}
          options={[{ value: 'off', label: 'Desligado' }, { value: 'on', label: 'Ligado' }]}
        />
        <label>
          <span>Operações vizinhas<Help text="Quantas operações a mais o grafo junta aos resultados, começando pelas mais próximas." /></span>
          <input
            type="number" min={0} max={10} value={value.related} disabled={disabled || !graphOn}
            onChange={(e) => onChange({ ...value, related: clamp(Number(e.target.value), 0, 10) })}
          />
        </label>
        <label>
          <span>Distância máxima (passos)<Help text="Até quantos passos no grafo uma vizinha pode estar. 2 passos é o mesmo recurso, 3 é o recurso pai ou filho." /></span>
          <input
            type="number" min={1} max={5} value={value.hops} disabled={disabled || value.related === 0}
            onChange={(e) => onChange({ ...value, hops: clamp(Number(e.target.value), 1, 5) })}
          />
        </label>
        <Choice
          label="Ordem de chamadas" value={value.plan ? 'on' : 'off'} disabled={disabled || !graphOn}
          help="Diz ao modelo o que chamar antes quando a operação precisa de um id que só outra chamada devolve. Exemplo: listar as releases antes de apagar uma."
          onChange={(v) => onChange({ ...value, plan: v === 'on' })}
          options={[{ value: 'off', label: 'Desligada' }, { value: 'on', label: 'Ligada' }]}
        />
      </div>
    </form>
  )
}

function Choice<T extends string>({ label, help, options, value, onChange, disabled }: {
  label: string
  help?: string
  options: { value: T; label: string }[]
  value: T
  onChange: (v: T) => void
  disabled: boolean
}) {
  return (
    <div className="choice" role="group" aria-label={label}>
      <span>{label}{help && <Help text={help} />}</span>
      <div className="choice-options">
        {options.map((o) => (
          <button key={o.value} type="button" aria-pressed={o.value === value} disabled={disabled} onClick={() => onChange(o.value)}>
            {o.label}
          </button>
        ))}
      </div>
    </div>
  )
}

function Help({ text }: { text: string }) {
  return (
    <span className="help">
      <span className="help-mark" tabIndex={0} role="img" aria-label={text}>?</span>
      <span className="help-tip" aria-hidden="true">{text}</span>
    </span>
  )
}

function sameOpts(a: Pick<SearchOptions, 'config' | 'select' | 'order_strategy'>, b: SearchOptions) {
  return a.config === b.config && a.select === b.select && (a.select !== 'sort' || a.order_strategy === b.order_strategy)
}

function Retrieval({ turn }: { turn: Turn }) {
  const tool = turn.tools[turn.tools.length - 1]
  const query = tool ? String(tool.args.query ?? '') : ''
  const answer = turn.pending ? '' : turn.answer
  const ops = tool?.result ? collectOps(tool.result, answer) : []
  return (
    <ol className="steps">
      <li>
        <p className="step-title">O modelo transformou a pergunta em busca</p>
        {tool ? (
          <p className="step-body">“{turn.question}” virou <code className="chip">{query}</code></p>
        ) : (
          <p className="step-body hint">{turn.pending ? 'Esperando o modelo montar a busca…' : 'A busca não foi feita.'}</p>
        )}
      </li>
      <li>
        <div className="step-head">
          <p className="step-title">O que foi entregue ao modelo, na árvore da API</p>
          {ops.length > 0 && (
            <p className="legend">
              {ops.some((o) => o.rank !== undefined) && <span><b className="mark-rank">[1]</b> posição na busca</span>}
              {ops.some((o) => o.hops !== undefined && o.rank === undefined) && <span><b className="mark-graph">◇</b> trazida pelo grafo</span>}
              {ops.some((o) => o.prerequisite) && <span><b className="mark-plan">antes</b> chamar antes, para pegar o id</span>}
              {ops.some((o) => o.answer) && <span><b className="answer-tag">resposta</b> a operação usada na resposta</span>}
            </p>
          )}
        </div>
        {tool?.result ? <PathTree output={tool.result} answer={answer} /> : (
          <p className="step-body hint">{tool ? 'Buscando…' : 'Nada recuperado ainda.'}</p>
        )}
        {tool?.result?.plans && tool.result.plans.length > 0 && <Plans plans={tool.result.plans} />}
      </li>
      <li>
        <p className="step-title">Onde foi o tempo</p>
        {tool?.result && turn.stats ? <TimeBar output={tool.result} llmMs={turn.stats.llm_ms} tokensIn={turn.stats.prompt_tokens} tokensOut={turn.stats.output_tokens} /> : (
          <p className="step-body hint">Aparece quando a resposta termina.</p>
        )}
      </li>
    </ol>
  )
}

function PathTree({ output, answer }: { output: SearchOutput; answer: string }) {
  const [open, setOpen] = useState<string | null>(null)
  const ops = collectOps(output, answer)
  if (ops.length === 0) return <p className="step-body hint">A busca não achou nenhuma operação com essas palavras.</p>
  const top = Math.max(...ops.map((o) => o.score ?? 0)) || 1
  const rows = buildRows(ops)
  return (
    <ul className="tree">
      {rows.map((row) => (
        <li key={row.key} className={row.kind === 'op' && row.op.answer ? 'is-answer' : ''}>
          <div className="tree-row">
            {Array.from({ length: row.depth }, (_, i) => <span key={i} className="rail" aria-hidden="true" />)}
            {row.kind === 'segment' ? (
              <code className="segment">{row.label}</code>
            ) : (
              <TreeOpRow op={row.op} top={top} open={open === row.op.key} toggle={() => setOpen(open === row.op.key ? null : row.op.key)} />
            )}
          </div>
          {row.kind === 'op' && open === row.op.key && row.op.text && (
            <div className="op-text" style={{ ['--depth' as string]: row.depth }}>
              <p>{row.op.text}</p>
              <a href={row.op.docsUrl} target="_blank" rel="noreferrer">Abrir a documentação</a>
            </div>
          )}
        </li>
      ))}
    </ul>
  )
}

function TreeOpRow({ op, top, open, toggle }: { op: TreeOp; top: number; open: boolean; toggle: () => void }) {
  const label = (
    <>
      <span className={'method m-' + op.method.toLowerCase()}>{op.method}</span>
      <span className="op-summary">{op.summary || op.path}</span>
    </>
  )
  return (
    <>
      {op.text ? (
        <button type="button" className="op-name" onClick={toggle} aria-expanded={open}>{label}</button>
      ) : op.docsUrl ? (
        <a className="op-name" href={op.docsUrl} target="_blank" rel="noreferrer">{label}</a>
      ) : (
        <span className="op-name">{label}</span>
      )}
      {op.answer && <span className="answer-tag">resposta</span>}
      {op.prerequisite && <span className="mark-plan">antes</span>}
      <span className="marks">
        {op.rank !== undefined ? <b className="mark-rank">[{op.rank}]</b> : op.hops !== undefined ? <b className="mark-graph" title={`a ${op.hops} passos`}>◇</b> : null}
      </span>
      <span className="bar" aria-hidden="true">
        {op.score !== undefined && <i style={{ width: `${(100 * op.score) / top}%` }} />}
      </span>
    </>
  )
}

// Plans junta os resultados que têm os mesmos pré-requisitos: com "cancel
// workflow run" os 5 resultados pedem o mesmo GET antes, e 5 listas iguais
// só repetiriam a mesma linha.
function Plans({ plans }: { plans: CallPlan[] }) {
  const groups = new Map<string, { ranks: number[]; steps: CallPlan['steps'] }>()
  for (const plan of plans) {
    const before = plan.steps.slice(0, -1)
    const key = before.map((s) => s.id).join(' ')
    const group = groups.get(key) ?? { ranks: [], steps: before }
    group.ranks.push(plan.rank)
    groups.set(key, group)
  }
  return (
    <div className="plans">
      {[...groups.entries()].map(([key, group]) => (
        <div key={key} className="plan">
          <span className="plan-for">Antes de usar {joinRanks(group.ranks)}, chame:</span>
          <ol>
            {group.steps.map((step) => (
              <li key={step.id}>
                <span className={'method m-' + step.method.toLowerCase()}>{step.method}</span> <code>{step.path}</code>
              </li>
            ))}
          </ol>
        </div>
      ))}
    </div>
  )
}

function joinRanks(ranks: number[]) {
  const labels = ranks.map((r) => `[${r}]`)
  return labels.length === 1 ? labels[0] : labels.slice(0, -1).join(', ') + ' e ' + labels[labels.length - 1]
}

function TimeBar({ output, llmMs, tokensIn, tokensOut }: { output: SearchOutput; llmMs: number; tokensIn: number; tokensOut: number }) {
  const searchUS = output.stats.query_us
  const graphUS = (output.related_stats?.expand_us ?? 0) + (output.plan_stats?.plan_us ?? 0)
  const usesGraph = Boolean(output.related_stats || output.plan_stats)
  const totalUS = searchUS + graphUS + llmMs * 1000
  const share = (100 * (searchUS + graphUS)) / totalUS
  return (
    <div className="time">
      <div className="time-bar" role="img" aria-label={`Busca ${us(searchUS)}, grafo ${us(graphUS)}, modelo ${seconds(llmMs)}`}>
        <span className="t-search" />
        {usesGraph && <span className="t-graph" />}
        <span className="t-model" />
      </div>
      <p className="time-legend">
        <span><i className="swatch t-search" /><b className="mark-rank">Busca</b> {us(searchUS)}</span>
        {usesGraph && <span><i className="swatch t-graph" /><b className="mark-graph">Grafo</b> {us(graphUS)}</span>}
        <span><i className="swatch t-model" /><b>Modelo</b> {seconds(llmMs)}, {tokensIn} tokens lidos e {tokensOut} escritos</span>
      </p>
      <p className="step-body hint">Busca e grafo somam {share < 0.01 ? 'menos de 0,01' : share.toFixed(2).replace('.', ',')}% do tempo da resposta.</p>
    </div>
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
  return n >= 1000 ? (n / 1000).toFixed(2).replace('.', ',') + ' ms' : n + ' µs'
}

function seconds(ms: number) {
  return (ms / 1000).toFixed(1).replace('.', ',') + ' s'
}

function clamp(n: number, lo: number, hi: number) {
  return Number.isFinite(n) ? Math.min(hi, Math.max(lo, n)) : lo
}

function gb(bytes: number) {
  return (bytes / 1e9).toFixed(1).replace('.', ',') + ' GB'
}
