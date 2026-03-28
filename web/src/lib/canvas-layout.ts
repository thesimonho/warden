import type { PanelGeometry } from '@/lib/canvas-store'

/** Input panel for layout computation. */
interface LayoutInput {
  id: string
  geometry: PanelGeometry
}

/** Output panel with computed geometry. */
interface LayoutOutput {
  id: string
  geometry: PanelGeometry
}

/** Sorts panels left-to-right, breaking ties by top-to-bottom. */
function sortByPosition(panels: LayoutInput[]): LayoutInput[] {
  return [...panels].sort((a, b) => {
    const dx = a.geometry.x - b.geometry.x
    return dx !== 0 ? dx : a.geometry.y - b.geometry.y
  })
}

/**
 * Arranges panels in a grid anchored at the bounding box top-left.
 *
 * Panels are sorted left-to-right (then top-to-bottom) before
 * assignment so the layout preserves spatial ordering.
 * Uses ceil(sqrt(count)) columns. Each panel keeps its own width/height.
 * Row height is the max height in that row, column width is the max
 * width in that column.
 */
export function layoutGrid(panels: LayoutInput[], gap: number): LayoutOutput[] {
  if (panels.length === 0) return []

  const sorted = sortByPosition(panels)
  const anchor = boundingBoxTopLeft(sorted)
  const cols = Math.ceil(Math.sqrt(sorted.length))

  const cells = sorted.map((panel, i) => ({
    panel,
    col: i % cols,
    row: Math.floor(i / cols),
  }))

  const colWidths = new Array<number>(cols).fill(0)
  const rows = Math.ceil(sorted.length / cols)
  const rowHeights = new Array<number>(rows).fill(0)

  for (const cell of cells) {
    colWidths[cell.col] = Math.max(colWidths[cell.col], cell.panel.geometry.width)
    rowHeights[cell.row] = Math.max(rowHeights[cell.row], cell.panel.geometry.height)
  }

  const colOffsets = cumulativeOffsets(colWidths, gap)
  const rowOffsets = cumulativeOffsets(rowHeights, gap)

  return cells.map(({ panel, col, row }) => ({
    id: panel.id,
    geometry: {
      ...panel.geometry,
      x: anchor.x + colOffsets[col],
      y: anchor.y + rowOffsets[row],
    },
  }))
}

/**
 * Arranges panels in a horizontal row, aligned at the top of the
 * bounding box. Sorted left-to-right by current position so the
 * layout preserves spatial ordering.
 */
export function layoutHorizontal(panels: LayoutInput[], gap: number): LayoutOutput[] {
  if (panels.length === 0) return []

  const sorted = sortByPosition(panels)
  const anchor = boundingBoxTopLeft(sorted)
  let currentX = anchor.x

  return sorted.map((panel) => {
    const result: LayoutOutput = {
      id: panel.id,
      geometry: {
        ...panel.geometry,
        x: currentX,
        y: anchor.y,
      },
    }
    currentX += panel.geometry.width + gap
    return result
  })
}

/**
 * Arranges panels in a vertical column, aligned at the left of the
 * bounding box. Sorted top-to-bottom by current position so the
 * layout preserves spatial ordering.
 */
export function layoutVertical(panels: LayoutInput[], gap: number): LayoutOutput[] {
  if (panels.length === 0) return []

  const sorted = sortByTopToBottom(panels)
  const anchor = boundingBoxTopLeft(sorted)
  let currentY = anchor.y

  return sorted.map((panel) => {
    const result: LayoutOutput = {
      id: panel.id,
      geometry: {
        ...panel.geometry,
        x: anchor.x,
        y: currentY,
      },
    }
    currentY += panel.geometry.height + gap
    return result
  })
}

/** Sorts panels top-to-bottom, breaking ties by left-to-right. */
function sortByTopToBottom(panels: LayoutInput[]): LayoutInput[] {
  return [...panels].sort((a, b) => {
    const dy = a.geometry.y - b.geometry.y
    return dy !== 0 ? dy : a.geometry.x - b.geometry.x
  })
}

/** Finds the top-left corner of the bounding box of all panels. */
function boundingBoxTopLeft(panels: LayoutInput[]): { x: number; y: number } {
  let minX = Infinity
  let minY = Infinity
  for (const panel of panels) {
    minX = Math.min(minX, panel.geometry.x)
    minY = Math.min(minY, panel.geometry.y)
  }
  return { x: minX, y: minY }
}

/** Converts an array of sizes into cumulative offsets with gaps. */
function cumulativeOffsets(sizes: number[], gap: number): number[] {
  const offsets: number[] = []
  let current = 0
  for (const size of sizes) {
    offsets.push(current)
    current += size + gap
  }
  return offsets
}
