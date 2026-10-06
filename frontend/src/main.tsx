import { StrictMode, useCallback, useEffect, useRef, useState } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'

const names = ['backend', 'mysql', 'kafka', 'redis'] as const
type ServiceName = typeof names[number]
type Service = { name: ServiceName; status: 'up' | 'down'; latency_ms: number; message: string }
type Snapshot = { status: 'ok' | 'degraded'; checked_at: string; services: Service[] }
const catalog: Record<ServiceName, { title: string; kind: string; detail: string; glyph: string }> = {
  backend: { title: 'Go API', kind: '应用服务', detail: '后端 HTTP 接口', glyph: '↗' },
  mysql: { title: 'MySQL', kind: '关系数据库', detail: '数据持久化连接', glyph: '▤' },
  kafka: { title: 'Kafka', kind: '消息队列', detail: '消息代理连接', glyph: '⇄' },
  redis: { title: 'Redis', kind: '缓存服务', detail: '缓存实例连接', glyph: '◇' },
}

function parseSnapshot(value: unknown): Snapshot {
  if (!value || typeof value !== 'object') throw new Error('状态接口返回了无效数据')
  const v = value as Partial<Snapshot>
  if (!['ok', 'degraded'].includes(v.status ?? '') || typeof v.checked_at !== 'string' || !Number.isFinite(Date.parse(v.checked_at)) || !Array.isArray(v.services)) throw new Error('状态接口返回了无效数据')
  if (v.services.length !== names.length || !names.every(name => v.services!.filter(s => s?.name === name).length === 1)) throw new Error('状态接口缺少服务信息')
  for (const s of v.services) {
    if (!s || !['up', 'down'].includes(s.status) || typeof s.latency_ms !== 'number' || !Number.isFinite(s.latency_ms) || s.latency_ms < 0 || typeof s.message !== 'string') throw new Error('状态接口返回了无效服务信息')
  }
  if ((v.status === 'ok') !== v.services.every(s => s.status === 'up')) throw new Error('状态接口的汇总与服务状态不一致')
  return v as Snapshot
}

function App() {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [automatic, setAutomatic] = useState(true)
  const request = useRef<AbortController | null>(null)
  const refresh = useCallback(async () => {
    if (request.current) return
    const controller = new AbortController()
    request.current = controller
    setLoading(true)
    const timeout = window.setTimeout(() => controller.abort('timeout'), 15000)
    try {
      const response = await fetch('/api/status', { signal: controller.signal, cache: 'no-store', headers: { Accept: 'application/json' } })
      // A degraded service may legitimately return 503 with a complete status payload.
      if (!response.ok && response.status !== 503) throw new Error(`请求失败（HTTP ${response.status}）`)
      const next = parseSnapshot(await response.json())
      if (!controller.signal.aborted) { setSnapshot(next); setError(null) }
    } catch (cause) {
      if (controller.signal.reason !== 'unmount') {
        setError(controller.signal.aborted ? '检查超时，请确认本机后端正在运行。' : cause instanceof Error ? cause.message : '无法连接状态接口')
      }
    } finally {
      window.clearTimeout(timeout)
      if (request.current === controller) { request.current = null; setLoading(false) }
    }
  }, [])
  useEffect(() => {
    void refresh()
    return () => { request.current?.abort('unmount'); request.current = null }
  }, [refresh])
  useEffect(() => {
    if (!automatic) return
    const timer = window.setInterval(() => { if (!document.hidden) void refresh() }, 15000)
    return () => window.clearInterval(timer)
  }, [automatic, refresh])
  const valid = snapshot !== null && error === null
  const count = valid ? snapshot.services.filter(s => s.status === 'up').length : 0
  const time = snapshot ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium', hour12: false }).format(new Date(snapshot.checked_at)) : '等待首次检查'
  return <div className="shell">
    <header className="topbar"><a className="brand" href="/" aria-label="飞图首页"><span className="brand-mark">飞</span><strong>飞图</strong><span className="brand-en">FEITU</span></a><span className="local"><i />本机开发环境</span></header>
    <main>
      <div className="eyebrow">WORKSPACE / OVERVIEW</div>
      <section className="page-heading"><div><h1>环境状态<span>.</span></h1><p>从连接开始，让每一步都有据可查。</p></div><button className="refresh" onClick={() => void refresh()} disabled={loading}><span className={loading ? 'spin' : ''} aria-hidden="true">↻</span>{loading ? '正在检查' : '刷新状态'}</button></section>
      <section className={`summary ${error ? 'failed' : valid && count === 4 ? 'healthy' : ''}`} aria-label="环境状态汇总" aria-busy={loading}>
        <div className="summary-icon" aria-hidden="true">{error ? '!' : valid && count === 4 ? '✓' : '⌁'}</div><div className="summary-copy"><div className="eyebrow">CONNECTION HEALTH</div><h2 aria-live="polite">{error ? '暂时无法获取环境状态' : !snapshot ? '正在建立第一份连接快照' : count === 4 ? '所有服务连接正常' : '部分服务需要检查'}</h2><p>{error ? '本次检查失败，服务状态未知。请检查后端进程及网络连接。' : '此处仅检查环境连通性，行情采集、处理与展示业务尚未实现。'}</p></div><div className="summary-count"><strong>{valid ? count : '—'}<span>/ 4</span></strong><span>可连接服务</span></div>
      </section>
      {error && <div className="error" role="alert"><strong>检查失败</strong><span>{error}</span></div>}
      <section aria-labelledby="services-heading"><div className="section-heading"><h2 id="services-heading">基础服务 <span>04</span></h2><span className="section-note">实时接口检查 · 无模拟数据</span></div><div className="services">{names.map((name, index) => {
        const meta = catalog[name]
        const service = valid ? snapshot.services.find(s => s.name === name) : undefined
        return <article className="service" key={name}><div className="card-top"><span className={`service-glyph glyph-${name}`} aria-hidden="true">{meta.glyph}</span><span className={`badge ${service?.status ?? 'unknown'}`}><i />{service?.status === 'up' ? '连接正常' : service?.status === 'down' ? '连接异常' : loading ? '检查中' : '状态未知'}</span></div><div className="service-kind">{meta.kind}</div><h3>{meta.title}</h3><p className="service-description">{meta.detail}</p><div className="latency"><span>检查耗时</span><strong>{service ? service.latency_ms.toFixed(1) : '—'}{service && <small> ms</small>}</strong></div><p className="message">{service?.message || (error ? '无法获取最新结果' : '等待检查结果…')}</p><span className="card-number" aria-hidden="true">0{index + 1}</span></article>
      })}</div></section>
      <div className="check-meta"><span>最近成功获取 <time dateTime={snapshot?.checked_at}>{time}</time>{error && snapshot && <strong> · 结果已过期</strong>}</span><label className="auto-refresh"><input type="checkbox" checked={automatic} onChange={event => setAutomatic(event.target.checked)} />每 15 秒自动刷新</label></div>
      <section className="next-step"><span className="next-label">当前阶段</span><div><h2>先接通环境，再构建数据链路。</h2><p>React 前端 → Go API → MySQL / Kafka / Redis</p></div><span className="phase">环境搭建</span></section>
    </main><footer><span>FEITU <span className="footer-dot">/</span> 数据平台工作台</span><span>LOCAL DEVELOPMENT</span></footer>
  </div>
}

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>)
