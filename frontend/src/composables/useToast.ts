import { reactive } from 'vue'

export type ToastType = 'info' | 'success' | 'error'
export interface ToastItem {
  id: number
  message: string
  type: ToastType
}

const toasts = reactive<ToastItem[]>([])
let seq = 1

function push(message: string, type: ToastType = 'info') {
  const id = seq++
  toasts.push({ id, message, type })
  window.setTimeout(() => dismiss(id), 3200)
}

function dismiss(id: number) {
  const idx = toasts.findIndex((t) => t.id === id)
  if (idx >= 0) toasts.splice(idx, 1)
}

export function useToast() {
  return { toasts, toast: push, dismiss }
}
