// Monta, a partir do que a busca devolveu, a árvore de paths que a tela
// desenha: a mesma estrutura que vira o grafo no searchEngine/graph.
import type { SearchOutput } from './types'

export type TreeOp = {
  key: string
  method: string
  path: string
  summary: string
  docsUrl: string
  text?: string
  rank?: number
  score?: number
  hops?: number
  prerequisite: boolean
  answer: boolean
}

export type TreeRow =
  | { kind: 'segment'; key: string; depth: number; label: string }
  | { kind: 'op'; key: string; depth: number; op: TreeOp }

type Node = { children: Map<string, Node>; ops: TreeOp[] }

const methodOrder = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']

// answeredEndpoint acha o primeiro "MÉTODO /path" na resposta do modelo, com
// ou sem crases: os modelos pequenos nem sempre formatam como código.
export function answeredEndpoint(answer: string): string | null {
  const match = answer.match(/\b(GET|POST|PUT|PATCH|DELETE)\s+(\/[^\s`)\]*,]+)/)
  return match ? match[1].toUpperCase() + ' ' + match[2] : null
}

export function collectOps(out: SearchOutput, answer: string): TreeOp[] {
  const byKey = new Map<string, TreeOp>()
  const chosen = answeredEndpoint(answer)
  const get = (method: string, path: string, summary: string, docsUrl: string) => {
    const key = method.toUpperCase() + ' ' + path
    let op = byKey.get(key)
    if (!op) {
      op = { key, method: method.toUpperCase(), path, summary, docsUrl, prerequisite: false, answer: key === chosen }
      byKey.set(key, op)
    }
    if (!op.summary) op.summary = summary
    if (!op.docsUrl) op.docsUrl = docsUrl
    return op
  }
  for (const r of out.results) {
    const op = get(r.method, r.path, r.summary, r.docs_url)
    op.rank = r.rank
    op.score = r.score
    op.text = r.text
  }
  for (const r of out.related ?? []) {
    get(r.method, r.path, r.summary, r.docs_url).hops = r.hops
  }
  for (const plan of out.plans ?? []) {
    for (const step of plan.steps.slice(0, -1)) {
      get(step.method, step.path, step.summary, '').prerequisite = true
    }
  }
  return [...byKey.values()]
}

export function buildRows(ops: TreeOp[]): TreeRow[] {
  const root: Node = { children: new Map(), ops: [] }
  for (const op of ops) {
    let node = root
    for (const segment of op.path.split('/').filter(Boolean)) {
      let child = node.children.get(segment)
      if (!child) {
        child = { children: new Map(), ops: [] }
        node.children.set(segment, child)
      }
      node = child
    }
    node.ops.push(op)
  }
  const rows: TreeRow[] = []
  walkChildren(root, 0, '', rows)
  return rows
}

// walkChildren junta trechos de path sem operação num rótulo só
// (/repos/{owner}/{repo}/releases) para a árvore não ficar funda à toa.
function walkChildren(node: Node, depth: number, prefix: string, rows: TreeRow[]) {
  for (const segment of [...node.children.keys()].sort()) {
    let child = node.children.get(segment)!
    let label = '/' + segment
    while (child.ops.length === 0 && child.children.size === 1) {
      const [next, only] = [...child.children.entries()][0]
      label += '/' + next
      child = only
    }
    const full = prefix + label
    rows.push({ kind: 'segment', key: 'seg ' + full, depth, label })
    const sorted = [...child.ops].sort((a, b) => methodOrder.indexOf(a.method) - methodOrder.indexOf(b.method))
    for (const op of sorted) rows.push({ kind: 'op', key: op.key, depth: depth + 1, op })
    walkChildren(child, depth + 1, full, rows)
  }
}
