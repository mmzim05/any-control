import { useEffect, useState } from 'react'
import type { OutputConfig } from '../hooks/useSimLink'
import { apiFetch } from '../hooks/useSimLink'

export default function Output() {
  const [cfg, setCfg] = useState<OutputConfig>({
    protocol: '',
    serial_port: '',
    audio_device: 'default',
    enabled: false,
  })
  const [serialPorts, setSerialPorts] = useState<string[]>([])
  const [audioDevices, setAudioDevices] = useState<string[]>(['default'])
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState('')

  useEffect(() => {
    apiFetch<OutputConfig>('/api/output/config').then(setCfg).catch(() => {})
    apiFetch<string[]>('/api/output/serial-ports').then(setSerialPorts).catch(() => {})
    apiFetch<string[]>('/api/output/audio-devices').then(setAudioDevices).catch(() => {})
  }, [])

  async function save() {
    setSaving(true)
    try {
      const saved = await apiFetch<OutputConfig>('/api/output/config', {
        method: 'PUT',
        body: JSON.stringify(cfg),
      })
      setCfg(saved)
      setStatus('Saved')
    } catch {
      setStatus('Save failed')
    } finally {
      setSaving(false)
      setTimeout(() => setStatus(''), 2000)
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Output</h1>
        <div className="flex items-center gap-3">
          {status && <span className="text-sm text-green-600 dark:text-green-400">{status}</span>}
          <button
            onClick={save}
            disabled={saving}
            className="px-4 py-1.5 bg-brand-500 hover:bg-brand-600 text-white rounded-lg text-sm font-medium disabled:opacity-50"
          >
            Save & Apply
          </button>
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-xl p-5 shadow-sm border border-gray-100 dark:border-gray-700 space-y-4">
        <div>
          <label className="block text-sm font-medium mb-1">Protocol</label>
          <select
            value={cfg.protocol}
            onChange={(e) => setCfg({ ...cfg, protocol: e.target.value as OutputConfig['protocol'] })}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-700 text-sm"
          >
            <option value="">— Select protocol —</option>
            <option value="crsf">CRSF / ExpressLRS (420100 baud, 150Hz)</option>
            <option value="sbus">SBUS (100000 baud, 50Hz) ⚠ needs hardware inverter</option>
            <option value="ppm">PPM Audio (via 3.5mm jack → trainer port)</option>
          </select>
        </div>

        {(cfg.protocol === 'crsf' || cfg.protocol === 'sbus') && (
          <div>
            <label className="block text-sm font-medium mb-1">Serial Port</label>
            <select
              value={cfg.serial_port}
              onChange={(e) => setCfg({ ...cfg, serial_port: e.target.value })}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-700 text-sm"
            >
              <option value="">— Select port —</option>
              {serialPorts.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
            {cfg.protocol === 'sbus' && (
              <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                SBUS requires an inverted signal. Use a hardware inverter (transistor / 74HC04) between TX and the receiver.
              </p>
            )}
          </div>
        )}

        {cfg.protocol === 'ppm' && (
          <div>
            <label className="block text-sm font-medium mb-1">Audio Device</label>
            <select
              value={cfg.audio_device}
              onChange={(e) => setCfg({ ...cfg, audio_device: e.target.value })}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-700 text-sm"
            >
              {audioDevices.map((d) => (
                <option key={d} value={d}>{d}</option>
              ))}
            </select>
            <p className="mt-1 text-xs text-gray-500">
              Connect 3.5mm audio out to the trainer port of your RC transmitter.
            </p>
          </div>
        )}

        <div className="flex items-center gap-3 pt-2">
          <input
            type="checkbox"
            id="enabled"
            checked={cfg.enabled}
            onChange={(e) => setCfg({ ...cfg, enabled: e.target.checked })}
          />
          <label htmlFor="enabled" className="text-sm font-medium">
            Enable output
          </label>
          <span
            className={`ml-auto text-xs px-2 py-0.5 rounded-full font-medium ${
              cfg.enabled
                ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                : 'bg-gray-100 text-gray-500 dark:bg-gray-700'
            }`}
          >
            {cfg.enabled ? 'Active' : 'Inactive'}
          </span>
        </div>
      </div>

    </div>
  )
}
