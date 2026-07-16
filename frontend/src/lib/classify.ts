// 进程类型启发式识别（从原 App.vue 迁移，ProcessesTab 与 PortsTab 共用）。
// Go/C# 编译产物无法 100% 识别，归为 other。

export type ProcessType = 'all' | 'node' | 'java' | 'python' | 'go' | 'csharp' | 'ai' | 'other'

export const PROCESS_TYPE_LABELS: { value: ProcessType; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'node', label: 'Node.js' },
  { value: 'java', label: 'Java' },
  { value: 'python', label: 'Python' },
  { value: 'go', label: 'Go' },
  { value: 'csharp', label: 'C#' },
  { value: 'ai', label: 'AI CLI' },
  { value: 'other', label: '其他' },
]

export function classifyProcess(name: string, path: string): ProcessType {
  const base = (name || '').toLowerCase().replace(/\.(exe|com)$/, '')
  const p = (path || '').toLowerCase()
  // AI CLI 优先：claude.exe / codex.exe 常经 npm 全局装在 nvm\node_modules 下，
  // 必须先按进程名判定，否则会被 node 的 nvm/node_modules 路径关键词误判。
  if (base === 'claude' || base === 'codex') return 'ai'
  if (['node', 'npm', 'npx', 'pnpm', 'yarn', 'bun'].includes(base) || p.includes('nodejs') || p.includes('node_modules') || p.includes('nvm')) return 'node'
  if (['java', 'javaw'].includes(base) || p.includes('\\jre') || p.includes('\\jdk') || p.includes('/jre') || p.includes('/jdk')) return 'java'
  if (['python', 'python3', 'pythonw', 'py'].includes(base) || p.includes('\\python') || p.includes('/python') || p.includes('\\anaconda') || p.includes('\\miniconda')) return 'python'
  if (base === 'go' || p.includes('\\go\\bin') || p.includes('/go/bin') || p.includes('go-build')) return 'go'
  if (base === 'dotnet' || p.includes('\\dotnet') || p.includes('/dotnet')) return 'csharp'
  return 'other'
}
