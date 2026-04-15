import { describe, expect, it } from 'vitest'

import { layoutGrid, layoutHorizontal, layoutVertical } from './canvas-layout'

const GAP = 16

function makePanel(id: string, x: number, y: number, w = 640, h = 480) {
  return { id, geometry: { x, y, width: w, height: h } }
}

describe('layoutGrid', () => {
  it('returns empty array for empty input', () => {
    expect(layoutGrid([], GAP)).toEqual([])
  })

  it('places a single panel at its current position', () => {
    const panels = [makePanel('a', 100, 200)]
    const result = layoutGrid(panels, GAP)
    expect(result).toHaveLength(1)
    expect(result[0].geometry.x).toBe(100)
    expect(result[0].geometry.y).toBe(200)
  })

  it('arranges 4 panels in a 2x2 grid sorted by position', () => {
    // Panels at scattered positions — sorted by x then y
    const panels = [
      makePanel('d', 800, 800, 110, 90),
      makePanel('a', 100, 100, 100, 80),
      makePanel('c', 700, 200, 80, 100),
      makePanel('b', 300, 500, 120, 60),
    ]
    const result = layoutGrid(panels, GAP)

    // Sorted by x: a(100), b(300), c(700), d(800)
    // Anchor at (100, 100) — min x/y of bounding box
    // cols = ceil(sqrt(4)) = 2
    // Grid slots: a=top-left, b=top-right, c=bottom-left, d=bottom-right
    // col widths: max(a.w=100, c.w=80)=100, max(b.w=120, d.w=110)=120
    // row heights: max(a.h=80, b.h=60)=80, max(c.h=100, d.h=90)=100
    const aResult = result.find((r) => r.id === 'a')!
    const bResult = result.find((r) => r.id === 'b')!
    const cResult = result.find((r) => r.id === 'c')!
    const dResult = result.find((r) => r.id === 'd')!

    expect(aResult.geometry).toEqual({ x: 100, y: 100, width: 100, height: 80 })
    expect(bResult.geometry).toEqual({ x: 100 + 100 + GAP, y: 100, width: 120, height: 60 })
    expect(cResult.geometry).toEqual({ x: 100, y: 100 + 80 + GAP, width: 80, height: 100 })
    expect(dResult.geometry).toEqual({
      x: 100 + 100 + GAP,
      y: 100 + 80 + GAP,
      width: 110,
      height: 90,
    })
  })

  it('preserves panel dimensions', () => {
    const panels = [makePanel('a', 0, 0, 300, 200), makePanel('b', 50, 0, 400, 100)]
    const result = layoutGrid(panels, GAP)
    const a = result.find((r) => r.id === 'a')!
    const b = result.find((r) => r.id === 'b')!
    expect(a.geometry.width).toBe(300)
    expect(a.geometry.height).toBe(200)
    expect(b.geometry.width).toBe(400)
    expect(b.geometry.height).toBe(100)
  })
})

describe('layoutHorizontal', () => {
  it('returns empty array for empty input', () => {
    expect(layoutHorizontal([], GAP)).toEqual([])
  })

  it('places panels in a row sorted left-to-right by position', () => {
    const panels = [
      makePanel('c', 400, 50, 80, 100),
      makePanel('a', 200, 100, 100, 80),
      makePanel('b', 50, 300, 120, 60),
    ]
    const result = layoutHorizontal(panels, GAP)

    // Sorted by x: b(50), a(200), c(400)
    // Anchor top-left: (50, 50) — min x, min y
    const b = result.find((r) => r.id === 'b')!
    const a = result.find((r) => r.id === 'a')!
    const c = result.find((r) => r.id === 'c')!

    expect(b.geometry).toEqual({ x: 50, y: 50, width: 120, height: 60 })
    expect(a.geometry).toEqual({ x: 50 + 120 + GAP, y: 50, width: 100, height: 80 })
    expect(c.geometry).toEqual({ x: 50 + 120 + GAP + 100 + GAP, y: 50, width: 80, height: 100 })
  })
})

describe('layoutVertical', () => {
  it('returns empty array for empty input', () => {
    expect(layoutVertical([], GAP)).toEqual([])
  })

  it('places panels in a column sorted top-to-bottom by position', () => {
    const panels = [
      makePanel('b', 50, 300, 120, 60),
      makePanel('c', 400, 50, 80, 100),
      makePanel('a', 200, 100, 100, 80),
    ]
    const result = layoutVertical(panels, GAP)

    // Sorted by y: c(50), a(100), b(300)
    // Anchor top-left: (50, 50) — min x, min y
    const c = result.find((r) => r.id === 'c')!
    const a = result.find((r) => r.id === 'a')!
    const b = result.find((r) => r.id === 'b')!

    expect(c.geometry).toEqual({ x: 50, y: 50, width: 80, height: 100 })
    expect(a.geometry).toEqual({ x: 50, y: 50 + 100 + GAP, width: 100, height: 80 })
    expect(b.geometry).toEqual({ x: 50, y: 50 + 100 + GAP + 80 + GAP, width: 120, height: 60 })
  })
})
