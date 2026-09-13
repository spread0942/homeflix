/** Episode / part label matching SeriesView. */
export function entryLabel(entry) {
  if (!entry) return null
  const bits = []
  if (entry.season != null) bits.push(`S${entry.season}`)
  if (entry.episode != null) bits.push(`E${entry.episode}`)
  if (!bits.length && entry.sort_order) bits.push(`Part ${entry.sort_order}`)
  return bits.length ? bits.join(' · ') : null
}

export function shortEntryLabel(entry) {
  if (!entry) return null
  if (entry.season != null || entry.episode != null) {
    let s = ''
    if (entry.season != null) s += `S${entry.season}`
    if (entry.episode != null) s += `E${entry.episode}`
    return s
  }
  if (entry.sort_order) return `Part ${entry.sort_order}`
  return entry.name
}

/** Group entries by season (API order preserved within each group). */
export function groupBySeason(entries = []) {
  const map = new Map()
  for (const e of entries) {
    const key = e.season == null ? 'parts' : `season-${e.season}`
    if (!map.has(key)) {
      map.set(key, {
        key,
        label: e.season == null ? 'Parts & specials' : `Season ${e.season}`,
        season: e.season,
        entries: [],
      })
    }
    map.get(key).entries.push(e)
  }
  return [...map.values()].sort((a, b) => {
    if (a.season == null && b.season == null) return 0
    if (a.season == null) return 1
    if (b.season == null) return -1
    return a.season - b.season
  })
}

/** Prev/next neighbors in the series entry order from the API. */
export function neighbors(entries = [], currentId) {
  const list = entries
  const index = list.findIndex((e) => e.id === currentId)
  return {
    index,
    prev: index > 0 ? list[index - 1] : null,
    next: index >= 0 && index < list.length - 1 ? list[index + 1] : null,
    list,
  }
}

export function isPlayable(entry) {
  return entry?.playback_status === 'ready'
}
