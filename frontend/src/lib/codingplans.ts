// Coding Plan 供应商元数据与用量展示工具。
// provider 枚举值与后端 codingplan.go 的常量一一对应（zhipu/kimi/minimax/zenmux）。

export interface ProviderRegion {
  value: string // 存入 CodingPlanAccount.baseUrl
  label: string
}

export interface ProviderMeta {
  label: string
  short: string // 头像字母
  color: string // 头像 / 强调色
  keyHint: string // 表单占位提示
  regions?: ProviderRegion[] // zhipu / minimax 的国内/国际站选择
  needURL?: boolean // zenmux 需要用户填写完整查询端点
  urlHint?: string
}

export const PROVIDERS: Record<string, ProviderMeta> = {
  zhipu: {
    label: '智谱 GLM',
    short: '智',
    color: '#3b82f6',
    keyHint: 'BigModel 开放平台 API Key',
    regions: [
      { value: 'https://open.bigmodel.cn', label: '国内站 open.bigmodel.cn' },
      { value: 'https://api.z.ai', label: '国际站 api.z.ai' },
    ],
  },
  kimi: {
    label: 'Kimi For Coding',
    short: 'K',
    color: '#8b5cf6',
    keyHint: 'Kimi API Key（api.kimi.com）',
  },
  minimax: {
    label: 'MiniMax',
    short: 'M',
    color: '#10b981',
    keyHint: 'MiniMax 开放平台 API Key',
    regions: [
      { value: 'https://api.minimaxi.com', label: '国内站 api.minimaxi.com' },
      { value: 'https://api.minimax.io', label: '国际站 api.minimax.io' },
    ],
  },
  zenmux: {
    label: 'ZenMux',
    short: 'Z',
    color: '#f59e0b',
    keyHint: 'ZenMux API Key',
    needURL: true,
    urlHint: '完整的用量查询端点 URL（https://…）',
  },
}

export function providerMeta(provider: string): ProviderMeta {
  return (
    PROVIDERS[provider] ?? {
      label: provider,
      short: provider.slice(0, 1).toUpperCase() || '?',
      color: 'var(--text-4)',
      keyHint: '',
    }
  )
}

export function defaultRegion(provider: string): string {
  return providerMeta(provider).regions?.[0]?.value ?? ''
}

// 用量分级阈值与 cc-switch 一致：<70% 正常、70-89% 偏高、≥90% 告警。
export type UsageLevel = 'ok' | 'warn' | 'danger'

export function usageLevel(pct: number): UsageLevel {
  if (pct >= 90) return 'danger'
  if (pct >= 70) return 'warn'
  return 'ok'
}

export const LEVEL_COLOR: Record<UsageLevel, string> = {
  ok: 'var(--emerald)',
  warn: 'var(--amber)',
  danger: 'var(--red)',
}

// 重置倒计时，格式与 cc-switch 一致：4h41m / 2d22h；未知或已过期返回空串。
export function countdown(resetsAt: string | null | undefined, now: number): string {
  if (!resetsAt) return ''
  const diff = new Date(resetsAt).getTime() - now
  if (Number.isNaN(diff) || diff <= 0) return ''
  const h = Math.floor(diff / 3_600_000)
  const m = Math.floor((diff % 3_600_000) / 60_000)
  if (h >= 24) return `${Math.floor(h / 24)}d${h % 24}h`
  if (h > 0) return `${h}h${m}m`
  return `${m}m`
}
