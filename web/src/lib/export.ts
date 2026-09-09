/**
 * 数据导出工具：将面板数据导出为 CSV / JSON 文件。
 * CSV 使用 BOM 头保证 Excel 正确识别 UTF-8 中文。
 */

type Cell = string | number | boolean | undefined | null

function escapeCsvCell(value: Cell): string {
  const s = value === undefined || value === null ? '' : String(value)
  // 包含逗号/引号/换行时用双引号包裹并转义内部引号
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`
  }
  return s
}

export function toCsv(headers: string[], rows: Cell[][]): string {
  const lines = [headers.map(escapeCsvCell).join(',')]
  for (const row of rows) {
    lines.push(row.map(escapeCsvCell).join(','))
  }
  return '\uFEFF' + lines.join('\n')
}

export function downloadFile(
  filename: string,
  content: string,
  mime = 'text/csv;charset=utf-8'
): void {
  const blob = new Blob([content], { type: mime })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

export function exportCsv(
  filename: string,
  headers: string[],
  rows: Cell[][]
): void {
  downloadFile(filename, toCsv(headers, rows))
}

export function exportJson(filename: string, data: unknown): void {
  downloadFile(
    filename,
    JSON.stringify(data, null, 2),
    'application/json;charset=utf-8'
  )
}

export function timestampSlug(): string {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}`
}
