type ViewName =
  | 'overview'
  | 'explore'
  | 'replay'
  | 'analytics'
  | 'relationships'
  | 'compare'
  | 'atlas'
  | 'datasets'
  | 'quality'
  | 'api-docs'
  | 'methodology'
  | 'about'

type Scenario = {
  id: string
  dataClass: string
  sourceDataset: string
  sourcePathRelation?: string | null
  sourcePet?: number | null
  sourceIntensity?: number | null
  agentCount: number
  keyAgentIds?: string[]
  trajectoriesAvailable?: boolean
}

type SearchResponse = {
  counts?: { research?: number; syntheticFixtures?: number }
  total?: number
  items?: Scenario[]
}

export function mountViewWidgets(
  root: HTMLElement,
  view: ViewName,
): () => void {
  const controller = new AbortController()
  const { signal } = controller
  let disposed = false
  let frame = 30
  let playing = false
  let speed = 1
  let timer: number | undefined
  let debounceTimer: number | undefined
  let gridVisible = true
  let mode = 'schematic'

  const query = <T extends Element>(selector: string) => root.querySelector<T>(selector)
  const on = (selector: string, event: string, listener: EventListener) => {
    query(selector)?.addEventListener(event, listener, { signal })
  }
  const escapeHTML = (value: unknown) => String(value ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[c]!)
  const updateTime = () => {
    const timecode = query<HTMLElement>('#timecode')
    const timeline = query<HTMLInputElement>('#timeline')
    if (timecode) timecode.textContent = `00:${String(Math.floor(frame / 10)).padStart(2, '0')}.${frame % 10}`
    if (timeline) {
      timeline.value = String(frame)
      const percent = (frame / 90) * 100
      timeline.style.background = `linear-gradient(90deg,var(--cyan) 0 ${percent}%,#36434e ${percent}% 100%)`
    }
  }
  const drawScene = (id: string) => {
    const host = query<HTMLElement>(`#${id}`)
    if (!host) return
    const w = host.clientWidth || 900
    const h = host.clientHeight || 380
    const cx = w / 2
    const cy = (h - (id === 'scene' ? 44 : 0)) / 2
    const scale = Math.min((w - 90) / 62, (h - 100) / 62)
    const road = 13 * scale
    const half = road / 2
    const t = frame / 10
    const a = { x: -24 + 5 * t, y: 0 }
    const b = { x: 0, y: -24 + 5 * t }
    const p = (x: number, y: number) => `${(cx + x * scale).toFixed(1)},${(cy - y * scale).toFixed(1)}`
    const grid = gridVisible && id === 'scene'
    const gridSize = Math.max(18, scale * 5)
    const background = grid
      ? `<pattern id="g" width="${gridSize}" height="${gridSize}" patternUnits="userSpaceOnUse"><path d="M ${gridSize} 0H0V${gridSize}" fill="none" stroke="#22303c" stroke-width=".7"/></pattern><rect width="100%" height="100%" fill="url(#g)"/>`
      : ''
    let crosswalk = ''
    for (let i = -5; i <= 5; i++) {
      crosswalk += `<path d="M ${cx - half - 10 * scale} ${cy + i * 2.3 * scale}h${5 * scale}" stroke="#80909a" stroke-width="1.3" opacity=".46"/><path d="M ${cx + half + 5 * scale} ${cy + i * 2.3 * scale}h${5 * scale}" stroke="#80909a" stroke-width="1.3" opacity=".46"/>`
    }
    const roadmark = `<path d="M ${cx - half} 0V${cy - 10 * scale} M ${cx - half} ${cy + 10 * scale}V${h} M ${cx + half} 0V${cy - 10 * scale} M ${cx + half} ${cy + 10 * scale}V${h}" stroke="#63717c" stroke-width="1" stroke-dasharray="8 9" opacity=".55"/><path d="M 0 ${cy + half}H${cx - 10 * scale} M ${cx + 10 * scale} ${cy + half}H${w} M 0 ${cy - half}H${cx - 10 * scale} M ${cx + 10 * scale} ${cy - half}H${w}" stroke="#63717c" stroke-width="1" stroke-dasharray="8 9" opacity=".55"/>`
    const vehicle = (x: number, y: number, color: string, rotation: number, label: string) =>
      `<g transform="translate(${p(x, y)}) rotate(${rotation})"><rect x="${-2.4 * scale}" y="${-1.05 * scale}" width="${4.8 * scale}" height="${2.1 * scale}" rx="${0.4 * scale}" fill="${color}" stroke="#e8f1f5" stroke-width="1.2"/><rect x="${-0.65 * scale}" y="${-0.79 * scale}" width="${1.55 * scale}" height="${1.58 * scale}" rx="${0.25 * scale}" fill="#0b1720" opacity=".84"/><path d="M ${1.2 * scale} 0h${0.8 * scale}" stroke="#f4f5e8" stroke-width="1.5"/><text x="0" y="${-1.6 * scale}" fill="#d4e1e8" font-size="${Math.max(8, scale * 1.2)}" text-anchor="middle" font-family="DM Mono">${label}</text></g>`
    const trail = (x: number, y: number, color: string, horizontal: boolean) => {
      const points: string[] = []
      for (let i = 0; i < 6; i++) {
        const distance = (5 - i) * 2.3
        points.push(horizontal ? p(x - distance, y) : p(x, y - distance))
      }
      return `<polyline points="${points.join(' ')}" fill="none" stroke="${color}" stroke-width="${1.5 * scale}" opacity=".30" stroke-linecap="round" stroke-dasharray="4 4"/>`
    }
    const conflict = `<circle cx="${cx}" cy="${cy}" r="${3 * scale}" fill="#df9852" fill-opacity=".12" stroke="#e7a45e" stroke-opacity=".78" stroke-width="1" stroke-dasharray="4 3"/><circle cx="${cx}" cy="${cy}" r="2.5" fill="#f0b86d"/><path d="M ${cx - 5 * scale} ${cy}h${10 * scale}M ${cx} ${cy - 5 * scale}v${10 * scale}" stroke="#f0b86d" opacity=".45" stroke-width=".7"/>`
    const engineering = mode === 'engineering' && id === 'scene'
    host.innerHTML = `<svg viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" aria-hidden="true"><defs><linearGradient id="asphalt" x2="0" y2="1"><stop stop-color="#1b2731"/><stop offset="1" stop-color="#151f28"/></linearGradient></defs>${background}<rect x="${cx - half}" y="0" width="${road}" height="${h}" fill="url(#asphalt)"/><rect x="0" y="${cy - half}" width="${w}" height="${road}" fill="url(#asphalt)"/><path d="M 0 ${cy - half}H${cx - half} M ${cx + half} ${cy - half}H${w} M 0 ${cy + half}H${cx - half} M ${cx + half} ${cy + half}H${w} M ${cx - half} 0V${cy - half} M ${cx - half} ${cy + half}V${h} M ${cx + half} 0V${cy - half} M ${cx + half} ${cy + half}V${h}" stroke="#66737e" stroke-width="1.1" opacity=".7" fill="none"/>${roadmark}${crosswalk}${engineering ? `<g fill="#55c5e5" fill-opacity=".22" stroke="#62c8e5" stroke-opacity=".65" stroke-width=".7"><rect x="${cx - half - 2 * scale}" y="${cy - 3.5 * scale}" width="${7 * scale}" height="${7 * scale}"/><rect x="${cx - 3.5 * scale}" y="${cy - half - 2 * scale}" width="${7 * scale}" height="${7 * scale}"/></g><g fill="#9aabb8" font-family="DM Mono" font-size="9"><text x="${cx + 5 * scale}" y="${cy - 4 * scale}">ZONE / 01</text><text x="${cx - half - 12 * scale}" y="${cy - 5 * scale}">ENTRY</text><text x="${cx + 5 * scale}" y="${cy + 7 * scale}">EXIT</text></g>` : ''}${conflict}${trail(a.x, a.y, '#58c8e7', true)}${trail(b.x, b.y, '#c1a5f3', false)}${vehicle(a.x, a.y, '#42b6d8', 0, 'A')}${vehicle(b.x, b.y, '#a587df', -90, 'B')}<g fill="#80909e" font-family="DM Mono" font-size="8"><text x="${w - 36}" y="${h - 58}">X →</text><text x="${w - 34}" y="${h - 70}">↑ Y</text></g><path d="M ${w - 48} ${h - 78}v24m-3-3 3 3 3-3" fill="none" stroke="#81909d" stroke-width="1"/></svg>`
    updateTime()
  }
  const activeScene = () => view === 'replay' ? 'replay-scene' : 'scene'
  const setFrame = (next: number) => {
    frame = (next + 91) % 91
    updateTime()
    drawScene(activeScene())
  }
  const togglePlay = () => {
    playing = !playing
    const button = query<HTMLButtonElement>('#play-toggle')
    if (button) {
      button.textContent = playing ? 'Ⅱ' : '▶'
      button.setAttribute('aria-label', playing ? 'Pause' : 'Play')
    }
    window.clearInterval(timer)
    if (playing) {
      timer = window.setInterval(() => setFrame((frame + 1) % 91), 100 / speed)
    }
  }
  const download = (data: unknown, filename: string) => {
    const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = filename
    anchor.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 0)
  }

  if (view === 'overview' || view === 'replay') {
    drawScene(view === 'replay' ? 'replay-scene' : 'scene')
    on('#timeline', 'input', ((event: Event) => setFrame(+(event.target as HTMLInputElement).value)) as EventListener)
    on('#play-toggle', 'click', togglePlay)
    on('#step-back', 'click', () => setFrame(frame - 1))
    on('#step-forward', 'click', () => setFrame(frame + 1))
    on('#speed-button', 'click', (event) => {
      speed = speed === 1 ? 2 : speed === 2 ? 0.5 : 1
      ;(event.currentTarget as HTMLButtonElement).textContent = `${speed}×`
      if (playing) {
        playing = false
        togglePlay()
      }
    })
    on('#grid-toggle', 'click', (event) => {
      gridVisible = !gridVisible
      ;(event.currentTarget as HTMLButtonElement).classList.toggle('active', gridVisible)
      drawScene('scene')
    })
    on('#reset-camera', 'click', () => setFrame(30))
    root.querySelectorAll<HTMLButtonElement>('.mode').forEach((button) => {
      button.addEventListener('click', () => {
        mode = button.dataset.mode ?? 'schematic'
        root.querySelectorAll('.mode').forEach((other) => other.classList.toggle('active', other === button))
        drawScene('scene')
      }, { signal })
    })
    const onKey = (event: KeyboardEvent) => {
      if (!root.isConnected || event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement || event.target instanceof HTMLSelectElement) return
      if (event.code === 'Space') { event.preventDefault(); togglePlay() }
      else if (event.key === 'ArrowLeft') setFrame(frame - 1)
      else if (event.key === 'ArrowRight') setFrame(frame + 1)
      else if (event.key === 'Home') setFrame(0)
      else if (event.key === 'End') setFrame(90)
      else if (event.key.toLowerCase() === 'r') setFrame(30)
      else if (event.key === 'Escape' && playing) togglePlay()
      else if (event.key.toLowerCase() === 'f') root.querySelector<HTMLElement>('.scene-frame,.replay-panel')?.requestFullscreen?.()
    }
    window.addEventListener('keydown', onKey, { signal })
    window.addEventListener('resize', () => drawScene(activeScene()), { signal })
  }

  if (view === 'explore') {
    const search = query<HTMLInputElement>('#scenario-search')!
    const pet = query<HTMLInputElement>('#pet-filter')!
    const min = query<HTMLSelectElement>('#agents-filter')!
    const fixtures = query<HTMLInputElement>('#fixtures-filter')!
    const dataset = query<HTMLSelectElement>('#dataset-filter')!
    let descending = false
    let requestId = 0
    let filters: Record<string, unknown> = {}
    let currentReport: Record<string, unknown> = {}
    const fetchDatasets = async () => {
      try {
        const response = await fetch('/v1/datasets', { signal })
        if (!response.ok) return
        const rows = await response.json() as { id: string }[]
        if (disposed) return
        for (const row of rows) {
          const option = document.createElement('option')
          option.value = row.id
          option.textContent = row.id
          dataset.append(option)
        }
      } catch { /* The empty catalog explains that no release is loaded. */ }
    }
    void fetchDatasets()
    const update = () => {
      window.clearTimeout(debounceTimer)
      debounceTimer = window.setTimeout(async () => {
        const id = ++requestId
        const value = query<HTMLOutputElement>('#pet-value')
        if (value) value.value = pet.value === '5' ? 'Any' : `< ${pet.value}s`
        const params = new URLSearchParams()
        if (fixtures.checked) params.set('includeFixtures', 'true')
        if (search.value.trim()) params.set('q', search.value.trim())
        if (dataset.value) params.set('dataset', dataset.value)
        if (+min.value > 0) params.set('agentCountMin', min.value)
        if (pet.value !== '5') params.set('sourcePetLt', pet.value)
        params.set('sort', 'sourcePet')
        params.set('direction', descending ? 'desc' : 'asc')
        params.set('limit', '50')
        const count = query<HTMLElement>('#result-count')
        if (count) count.textContent = 'SEARCHING INDEX…'
        filters = {
          sourceDataset: dataset.value || null,
          sourcePet: { metric: 'source_pet', operator: 'lt', value: pet.value === '5' ? null : +pet.value },
          minimumAgents: +min.value,
          query: search.value,
          includeFixtures: fixtures.checked,
        }
        try {
          const response = await fetch(`/v1/scenarios?${params}`, { signal })
          if (!response.ok) throw new Error('Scenario search failed')
          const result = await response.json() as SearchResponse
          if (disposed || id !== requestId) return
          const research = result.counts?.research ?? result.total ?? 0
          const synthetic = result.counts?.syntheticFixtures ?? 0
          if (count) count.textContent = `${research} RESEARCH RECORDS · ${synthetic} FIXTURES`
          const banner = query<HTMLElement>('.catalog-banner')
          if (banner) banner.innerHTML = research
            ? `<span class="status-dot"></span><div><b>${research.toLocaleString()} research records match this search.</b><span> Replay availability and missing annotations are shown per record.</span></div>`
            : '<span class="status-dot"></span><div><b>No research records match this search.</b><span> Synthetic fixtures are displayed separately when enabled.</span></div>'
          const list = query<HTMLElement>('#results-list')
          if (!list) return
          if (!result.items?.length) {
            list.innerHTML = `<div class="empty-mark">⌕</div><h3>No matching scenarios</h3><p>The filtered population is empty. Adjust the filters or import a validated source release.</p>${fixtures.checked ? '' : '<p class="fixture-prompt">Enable “Include synthetic fixtures” to add the deterministic test scenario.</p>'}`
            return
          }
          list.innerHTML = result.items.map(renderScenario).join('')
        } catch {
          if (disposed || id !== requestId) return
          if (count) count.textContent = 'INDEX UNAVAILABLE'
          const list = query<HTMLElement>('#results-list')
          if (list) list.innerHTML = '<div class="empty-mark">!</div><h3>Scenario index unavailable</h3><p>Start the local API with <code>make dev</code> and retry.</p>'
        }
      }, 120)
    }
    ;[search, pet, min, fixtures, dataset].forEach((control) => {
      control.addEventListener('input', update, { signal })
      control.addEventListener('change', update, { signal })
    })
    on('#clear-filters', 'click', () => {
      search.value = ''; pet.value = '5'; min.value = '0'; fixtures.checked = false; dataset.value = ''; update()
    })
    on('#sort-button', 'click', (event) => {
      descending = !descending
      ;(event.currentTarget as HTMLButtonElement).textContent = descending ? 'SORT · PET ↓' : 'SORT · PET ↑'
      update()
    })
    on('#export-query', 'click', () => {
      currentReport = { filters, datasetRelease: 'reported by API on request', timestamp: new Date().toISOString() }
      download(currentReport, 'atlas-query.json')
    })
    update()
  }

  if (view === 'datasets') {
    void fetch('/v1/datasets', { signal }).then((response) => response.json()).then((rows: { id: string; scenarioCount: number; sourcePetCoverage: number; trajectoryCount: number; mapCount: number }[]) => {
      if (disposed) return
      const host = query<HTMLElement>('#dataset-index')
      if (host) {
        host.innerHTML = rows.length
          ? `<div class="imported-datasets"><div class="section-kicker">IMPORTED RESEARCH DATASETS</div>${rows.map((dataset) => `<div><b>${escapeHTML(dataset.id)}</b><span>${dataset.scenarioCount.toLocaleString()} scenarios · ${dataset.sourcePetCoverage.toLocaleString()} source PET values · ${dataset.trajectoryCount.toLocaleString()} trajectories · ${dataset.mapCount.toLocaleString()} maps</span></div>`).join('')}</div>`
          : '<div class="no-imported-datasets">No dataset records have been indexed yet.</div>'
      }
      if (rows.length) {
        const total = rows.reduce((s, r) => s + r.scenarioCount, 0)
        const petTotal = rows.reduce((s, r) => s + r.sourcePetCoverage, 0)
        const cov = query<HTMLElement>('.coverage-grid')
        if (cov) {
          cov.innerHTML = `<div><span>IMPORTED SCENARIOS</span><b>${total}</b></div><div><span>TRAJECTORIES</span><b>—</b></div><div><span>MAP COVERAGE</span><b>—</b></div><div><span>MEASUREMENT COVERAGE</span><b>${petTotal}</b></div>`
        }
      }
    }).catch(() => {})
  }

  if (view === 'analytics') {
    void fetch('/v1/analytics/query', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ filters: {} }), signal })
      .then((response) => response.json())
      .then((data: { sourcePET?: { count?: number; min?: number; max?: number }; populationCount?: number }) => {
        if (disposed) return
        const node = query<HTMLElement>('#analytics-pet-caption')
        if (node) {
          node.innerHTML = !data.sourcePET?.count
            ? 'No research measurements<br><small>Waiting for validated dataset import</small>'
            : `${data.sourcePET.count.toLocaleString()} source PET values<br><small>range ${data.sourcePET.min}s – ${data.sourcePET.max}s · selected population ${data.populationCount}</small>`
        }
        if (data.sourcePET?.count) {
          const banner = query<HTMLElement>('.catalog-banner')
          if (banner) {
            banner.innerHTML = `<span class="status-dot"></span><div><b>${data.sourcePET.count} research records analyzed.</b><span> Population aggregates computed from active release.</span></div>`
          }
        }
      }).catch(() => {})
    void fetch('/v1/relationships', { signal }).then((response) => response.json()).then((rows: { count: number; pathCategory?: string; pathRelation?: string }[]) => {
      if (disposed || !rows.length) return
      const matrix = query<HTMLElement>('#relationship-matrix')
      if (matrix) matrix.innerHTML = rows.slice(0, 9).map((row) => `<span title="${escapeHTML(row.pathCategory || 'unknown')} / ${escapeHTML(row.pathRelation || 'unknown')}">${Number(row.count).toLocaleString()}</span>`).join('')
      const note = query<HTMLElement>('.chart-note')
      if (note) note.textContent = `${rows.reduce((sum, r) => sum + r.count, 0)} total interaction pairs categorized across ${rows.length} path relation classes.`
    }).catch(() => {})
  }

  if (view === 'relationships') {
    void fetch('/v1/relationships', { signal }).then((response) => response.json()).then((rows: { count: number; pathCategory?: string; pathRelation?: string }[]) => {
      if (disposed || !rows.length) return
      const matrix = query<HTMLElement>('#relationship-matrix') || query<HTMLElement>('.blank-matrix')
      if (matrix) matrix.innerHTML = rows.slice(0, 9).map((row) => `<span title="${escapeHTML(row.pathCategory || 'unknown')} / ${escapeHTML(row.pathRelation || 'unknown')}">${Number(row.count).toLocaleString()}</span>`).join('')
      const banner = query<HTMLElement>('.catalog-banner')
      if (banner) {
        const total = rows.reduce((s, r) => s + r.count, 0)
        banner.innerHTML = `<span class="status-dot"></span><div><b>${total} annotated interaction pairs indexed.</b><span> Categorized across ${rows.length} path relation classes with verified pair semantics.</span></div>`
      }
    }).catch(() => {})
  }

  if (view === 'quality') {
    let report: Record<string, unknown> = { researchRecords: 0, errors: 0, warnings: 0 }
    void fetch('/v1/quality', { signal }).then((response) => response.json()).then((data: Record<string, number>) => {
      if (disposed) return
      report = data
      const grid = query<HTMLElement>('#quality-grid')
      if (grid) grid.innerHTML = `<div><span>RESEARCH RECORDS CHECKED</span><b>${data.researchRecords}</b></div><div><span>VALIDATION ERRORS</span><b>${data.errors ?? 0}</b></div><div><span>QUALITY WARNINGS</span><b>${data.warnings ?? 0}</b></div><div><span>MAP COVERAGE</span><b>—</b></div>`
      if ((data.researchRecords ?? 0) > 0) {
        const banner = query<HTMLElement>('.catalog-banner')
        if (banner) {
          banner.innerHTML = `<span class="status-dot"></span><div><b>${data.researchRecords} research records audited.</b><span> Clean quality score with ${data.errors ?? 0} schema errors and ${data.warnings ?? 0} warnings.</span></div>`
        }
      }
    }).catch(() => {})
    on('#export-quality', 'click', () => download({ ...report, fixtureExcluded: true }, 'atlas-quality-report.json'))
  }

  function renderScenario(scenario: Scenario) {
    const pet = scenario.sourcePet == null ? '—' : `${Number(scenario.sourcePet).toFixed(2)} s`
    const intensity = scenario.sourceIntensity == null ? '—' : Number(scenario.sourceIntensity).toFixed(2)
    const pair = scenario.keyAgentIds?.join(' ↔ ') || 'Source key pair unavailable'
    if (scenario.dataClass === 'synthetic-fixture') {
      return `<button class="fixture-result" data-view="replay"><span class="fixture-tag"><span class="fixture-dot"></span> SYNTHETIC FIXTURE</span><b>Two-vehicle perpendicular crossing</b><span class="mono">${escapeHTML(scenario.id)} · local metric frame</span><span class="fixture-open">Open replay →</span></button>`
    }
    return `<article class="scenario-record"><div class="scenario-record-head"><span class="record-type">RESEARCH RECORD</span><span class="mono">${escapeHTML(scenario.id)}</span></div><b>${escapeHTML(scenario.sourceDataset)} · ${escapeHTML(scenario.sourcePathRelation || 'Path relation unavailable')}</b><div class="scenario-facts"><span>source PET <strong>${escapeHTML(pet)}</strong></span><span>intensity <strong>${escapeHTML(intensity)}</strong></span><span>agents <strong>${escapeHTML(scenario.agentCount)}</strong></span></div><div class="mono record-pair">KEY PAIR · ${escapeHTML(pair)}</div><a href="/v1/scenarios/${encodeURIComponent(scenario.id)}" target="_blank" rel="noreferrer">Open metadata record ↗</a>${scenario.trajectoriesAvailable ? '<span class="record-availability">TRAJECTORY ASSET AVAILABLE</span>' : '<span class="record-availability">METADATA ONLY · REPLAY UNAVAILABLE</span>'}</article>`
  }

  return () => {
    disposed = true
    controller.abort()
    window.clearTimeout(debounceTimer)
    window.clearInterval(timer)
  }
}
