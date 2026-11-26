import { useEffect, useMemo, useRef, useState, type MouseEvent } from 'react'
import pageSource from './views.html?raw'
import { mountViewWidgets } from './mountViewWidgets'

const views = [
  'overview',
  'explore',
  'replay',
  'analytics',
  'relationships',
  'compare',
  'atlas',
  'datasets',
  'quality',
  'api-docs',
  'methodology',
  'about',
] as const

type View = (typeof views)[number]

const titles: Record<View, string> = {
  overview: 'OVERVIEW',
  explore: 'SCENARIO EXPLORER',
  replay: 'REPLAY LABORATORY',
  analytics: 'ANALYTICS',
  relationships: 'RELATIONSHIPS',
  compare: 'COMPARE',
  atlas: 'SPATIAL ATLAS',
  datasets: 'DATASETS',
  quality: 'DATA QUALITY',
  'api-docs': 'API DOCS',
  methodology: 'METHODOLOGY',
  about: 'ABOUT',
}

const navigation: { view: View; label: string; icon: string }[] = [
  { view: 'overview', label: 'Overview', icon: '⌂' },
  { view: 'explore', label: 'Explore scenarios', icon: '⌕' },
  { view: 'replay', label: 'Replay laboratory', icon: '◉' },
  { view: 'analytics', label: 'Analytics', icon: '⌁' },
  { view: 'relationships', label: 'Relationships', icon: '⌘' },
  { view: 'compare', label: 'Compare', icon: '⇆' },
  { view: 'atlas', label: 'Spatial atlas', icon: '⊹' },
  { view: 'datasets', label: 'Datasets', icon: '▤' },
  { view: 'quality', label: 'Data quality', icon: '◌' },
  { view: 'api-docs', label: 'API docs', icon: '⌘' },
]

function readHash(): View {
  const name = window.location.hash.slice(1).split('?')[0]
  return views.find((view) => view === name) ?? 'overview'
}

function goTo(view: View) {
  if (window.location.hash !== `#${view}`) window.location.hash = view
}

function extractTemplates(source: string): Partial<Record<View, string>> {
  const document = new DOMParser().parseFromString(source, 'text/html')
  const templates: Partial<Record<View, string>> = {}
  document.querySelectorAll('template[id$="-template"]').forEach((template) => {
    const name = template.id.replace(/-template$/, '') as View
    if (views.includes(name)) templates[name] = template.innerHTML
  })
  return templates
}

export default function App() {
  const templates = useMemo(() => extractTemplates(pageSource), [])
  const [view, setView] = useState<View>(readHash)
  const [apiStatus, setApiStatus] = useState('CHECKING API')
  const [researchCount, setResearchCount] = useState(0)
  const [releaseInfo, setReleaseInfo] = useState({
    title: 'No research data imported',
    id: 'release / none',
  })
  const [lightTheme, setLightTheme] = useState(
    () => window.localStorage.getItem('tca-theme') === 'light',
  )
  const contentRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onHashChange = () => setView(readHash())
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    fetch('/v1/health', { signal: controller.signal })
      .then((response) => {
        if (!response.ok) throw new Error('API request failed')
        return response.json() as Promise<{ status?: string; researchScenarios?: number }>
      })
      .then((health) => {
        setApiStatus(health.status === 'ok' ? 'API CONNECTED' : 'API ISSUE')
        if (typeof health.researchScenarios === 'number') {
          setResearchCount(health.researchScenarios)
          if (health.researchScenarios > 0) {
            setReleaseInfo({
              title: `${health.researchScenarios} research scenario${health.researchScenarios === 1 ? '' : 's'}`,
              id: 'release / verified',
            })
          }
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) setApiStatus('API OFFLINE')
      })
    return () => controller.abort()
  }, [])

  useEffect(() => {
    document.documentElement.classList.toggle('light', lightTheme)
    window.localStorage.setItem('tca-theme', lightTheme ? 'light' : 'dark')
  }, [lightTheme])

  useEffect(() => {
    const root = contentRef.current
    if (!root) return
    return mountViewWidgets(root, view)
  }, [view])

  function handleContentClick(event: MouseEvent<HTMLDivElement>) {
    const target = (event.target as HTMLElement).closest<HTMLElement>('[data-view]')
    const next = target?.dataset.view
    if (next && views.includes(next as View)) {
      event.preventDefault()
      goTo(next as View)
    }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a className="brand" href="#overview" aria-label="Traffic Conflict Atlas home">
          <span className="brand-mark" aria-hidden="true">
            <svg viewBox="0 0 32 32">
              <path d="M4 24 13 15 19 21 28 8" />
              <path d="M4 8 12 16" />
              <circle cx="16" cy="18" r="3.2" />
            </svg>
          </span>
          <span className="brand-name">TRAFFIC<br /><b>CONFLICT ATLAS</b></span>
        </a>
        <div className="workspace-label">RESEARCH WORKSPACE</div>
        <nav className="primary-nav" aria-label="Main navigation">
          {navigation.map(({ view: itemView, label, icon }) => (
            <a
              className={`nav-item${view === itemView ? ' active' : ''}`}
              href={`#${itemView}`}
              key={itemView}
              aria-current={view === itemView ? 'page' : undefined}
            >
              <span className="nav-icon" aria-hidden="true">{icon}</span>
              {label}
              {itemView === 'explore' && <span className="nav-count">{researchCount}</span>}
            </a>
          ))}
        </nav>
        <div className="sidebar-spacer" />
        <div className="release-card">
          <div className="release-head"><span className="status-dot" /> LOCAL RELEASE</div>
          <div className="release-title">{releaseInfo.title}</div>
          <div className="release-id mono">{releaseInfo.id}</div>
          <a className="text-button" href="#datasets">Data sources &amp; licensing <span>↗</span></a>
        </div>
        <div className="sidebar-bottom">
          <a href="#methodology">Methodology</a>
          <a href="#about">About Atlas</a>
          <div className="version mono">ATLAS BUILD 0.1 · LOCAL</div>
        </div>
      </aside>
      <main className="main-area">
        <header className="topbar">
          <div className="breadcrumb"><span>ATLAS</span><i>/</i><b>{titles[view]}</b></div>
          <div className="top-actions">
            <span className="api-status"><span className="status-dot" /><span>{apiStatus}</span></span>
            <button
              className="icon-button"
              type="button"
              aria-label={`Switch to ${lightTheme ? 'dark' : 'light'} theme`}
              title={`Switch to ${lightTheme ? 'dark' : 'light'} theme`}
              onClick={() => setLightTheme((current) => !current)}
            >◐</button>
            <a href="/v1/openapi.json" className="api-link" target="_blank" rel="noreferrer">API <span>↗</span></a>
          </div>
        </header>
        <div
          className="content"
          id="content"
          ref={contentRef}
          onClick={handleContentClick}
          dangerouslySetInnerHTML={{ __html: templates[view] ?? templates.overview ?? '' }}
        />
      </main>
    </div>
  )
}
