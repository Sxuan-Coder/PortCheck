// 数值与字节格式化工具。

const NUMBER_FMT = new Intl.NumberFormat('zh-CN')

export function formatNumber(value: number): string {
  return NUMBER_FMT.format(value)
}

/** 把字节数格式化为最贴近的 GB/MB 显示。 */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 MB'
  const mb = bytes / (1024 * 1024)
  if (mb < 1024) return `${mb.toFixed(1)} MB`
  return `${(mb / 1024).toFixed(2)} GB`
}

/** GB 保留一位小数。 */
export function formatGB(gb: number): string {
  return `${gb.toFixed(1)} GB`
}

export function formatPercent(v: number, digits = 1): string {
  return `${v.toFixed(digits)}%`
}
