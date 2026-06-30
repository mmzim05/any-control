import { useCallback, useEffect, useState } from 'react'
import { useWebSocket } from './useWebSocket'

export type AxisInfo = { code: number; name: string; min: number; max: number }
export type ButtonInfo = { code: number; name: string }
export type Device = {
  id: string
  name: string
  vendor: number
  product: number
  axes: AxisInfo[]
  buttons: ButtonInfo[]
}

export type Transform = {
  scale: number
  offset: number
  deadzone: number
  expo: number
  reverse: boolean
}

export type MappingRule = {
  device_id: string
  code: number
  event_type: 0 | 1 // 0=axis 1=button
  channel: number
  transform: Transform
}

export type OutputConfig = {
  protocol: 'crsf' | 'sbus' | 'ppm' | ''
  serial_port: string
  audio_device: string
  failsafe: number[]
  enabled: boolean
}

export type Profile = {
  id: number
  name: string
  description: string
  created_at: string
  updated_at: string
  config: { rules: MappingRule[]; output: OutputConfig }
}

export const defaultTransform = (): Transform => ({
  scale: 1,
  offset: 0,
  deadzone: 0,
  expo: 0,
  reverse: false,
})

const API_BASE = ''

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  if (res.status === 204) return undefined as unknown as T
  return res.json()
}

export function useSimLink() {
  const [channels, setChannels] = useState<number[]>(new Array(32).fill(0))
  const [devices, setDevices] = useState<Device[]>([])
  const [rules, setRulesState] = useState<MappingRule[]>([])
  const [outputConfig, setOutputConfigState] = useState<OutputConfig>({
    protocol: '',
    serial_port: '',
    audio_device: 'default',
    failsafe: new Array(32).fill(0),
    enabled: false,
  })
  const [connected, setConnected] = useState(false)

  const wsConnected = useWebSocket((msg) => {
    if (msg.type === 'channels') setChannels(msg.payload as number[])
    if (msg.type === 'devices') setDevices(msg.payload as Device[])
  })

  useEffect(() => {
    setConnected(wsConnected)
  }, [wsConnected])

  // Load initial state
  useEffect(() => {
    apiFetch<OutputConfig>('/api/output/config').then(setOutputConfigState).catch(() => {})
    apiFetch<MappingRule[]>('/api/mapping/rules').catch(() => {}) // optional: may not exist yet
  }, [])

  const setRules = useCallback(async (newRules: MappingRule[]) => {
    await apiFetch('/api/mapping/rules', {
      method: 'PUT',
      body: JSON.stringify(newRules),
    })
    setRulesState(newRules)
  }, [])

  const setOutputConfig = useCallback(async (cfg: OutputConfig) => {
    const saved = await apiFetch<OutputConfig>('/api/output/config', {
      method: 'PUT',
      body: JSON.stringify(cfg),
    })
    setOutputConfigState(saved)
  }, [])

  return {
    channels,
    devices,
    rules,
    setRules,
    outputConfig,
    setOutputConfig,
    connected,
  }
}

export { apiFetch }
