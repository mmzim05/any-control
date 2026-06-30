import { useEffect, useState } from 'react'
import type { Device, MappingRule, Transform } from '../hooks/useSimLink'
import { apiFetch, defaultTransform } from '../hooks/useSimLink'

const NUM_CHANNELS = 32

function TransformEditor({
  transform,
  onChange,
}: {
  transform: Transform
  onChange: (t: Transform) => void
}) {
  const set = (k: keyof Transform, v: number | boolean) =>
    onChange({ ...transform, [k]: v })

  return (
    <div className="grid grid-cols-2 gap-2 text-xs mt-2">
      <label className="flex flex-col gap-1">
        Scale
        <input
          type="number"
          step="0.1"
          value={transform.scale}
          onChange={(e) => set('scale', parseFloat(e.target.value) || 1)}
          className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1 bg-transparent"
        />
      </label>
      <label className="flex flex-col gap-1">
        Offset
        <input
          type="number"
          step="0.01"
          value={transform.offset}
          onChange={(e) => set('offset', parseFloat(e.target.value) || 0)}
          className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1 bg-transparent"
        />
      </label>
      <label className="flex flex-col gap-1">
        Deadzone
        <input
          type="range"
          min="0"
          max="0.5"
          step="0.01"
          value={transform.deadzone}
          onChange={(e) => set('deadzone', parseFloat(e.target.value))}
        />
        <span>{(transform.deadzone * 100).toFixed(0)}%</span>
      </label>
      <label className="flex flex-col gap-1">
        Expo
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={transform.expo}
          onChange={(e) => set('expo', parseFloat(e.target.value))}
        />
        <span>{(transform.expo * 100).toFixed(0)}%</span>
      </label>
      <label className="flex items-center gap-2 col-span-2">
        <input
          type="checkbox"
          checked={transform.reverse}
          onChange={(e) => set('reverse', e.target.checked)}
        />
        Reverse
      </label>
    </div>
  )
}

export default function Mapping() {
  const [devices, setDevices] = useState<Device[]>([])
  const [rules, setRules] = useState<MappingRule[]>([])
  const [selected, setSelected] = useState<{
    deviceId: string
    code: number
    eventType: 0 | 1
  } | null>(null)
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState('')

  useEffect(() => {
    apiFetch<Device[]>('/api/devices').then(setDevices).catch(() => {})
  }, [])

  function getRule(ch: number): MappingRule | undefined {
    return rules.find((r) => r.channel === ch)
  }

  function assignToChannel(ch: number) {
    if (!selected) return
    const existing = rules.filter((r) => r.channel !== ch)
    const rule: MappingRule = {
      device_id: selected.deviceId,
      code: selected.code,
      event_type: selected.eventType,
      channel: ch,
      transform: defaultTransform(),
      failsafe: 0,
    }
    setRules([...existing, rule])
  }

  function updateTransform(ch: number, t: Transform) {
    setRules(rules.map((r) => (r.channel === ch ? { ...r, transform: t } : r)))
  }

  function updateFailsafe(ch: number, v: number) {
    const clamped = Math.max(-1, Math.min(1, isNaN(v) ? 0 : v))
    setRules(rules.map((r) => (r.channel === ch ? { ...r, failsafe: clamped } : r)))
  }

  function clearChannel(ch: number) {
    setRules(rules.filter((r) => r.channel !== ch))
  }

  async function save() {
    setSaving(true)
    try {
      await apiFetch('/api/mapping/rules', {
        method: 'PUT',
        body: JSON.stringify(rules),
      })
      setStatus('Saved')
    } catch {
      setStatus('Save failed')
    } finally {
      setSaving(false)
      setTimeout(() => setStatus(''), 2000)
    }
  }

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Mapping</h1>
        <div className="flex items-center gap-3">
          {status && <span className="text-sm text-green-600 dark:text-green-400">{status}</span>}
          <button
            onClick={save}
            disabled={saving}
            className="px-4 py-1.5 bg-brand-500 hover:bg-brand-600 text-white rounded-lg text-sm font-medium disabled:opacity-50"
          >
            Save
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Left: Device inputs */}
        <div className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700">
          <h2 className="text-sm font-semibold mb-3 text-gray-600 dark:text-gray-400">
            Input Sources — click to select
          </h2>
          {devices.length === 0 ? (
            <p className="text-sm text-gray-400">No devices connected</p>
          ) : (
            devices.map((d) => (
              <div key={d.id} className="mb-4">
                <p className="font-medium text-sm mb-2">{d.name}</p>
                <div className="space-y-1 pl-2">
                  {d.axes?.map((a) => {
                    const sel =
                      selected?.deviceId === d.id &&
                      selected?.code === a.code &&
                      selected?.eventType === 0
                    return (
                      <button
                        key={a.code}
                        onClick={() =>
                          setSelected({ deviceId: d.id, code: a.code, eventType: 0 })
                        }
                        className={`w-full text-left px-3 py-1 rounded text-xs transition-colors ${
                          sel
                            ? 'bg-brand-500 text-white'
                            : 'hover:bg-gray-100 dark:hover:bg-gray-700'
                        }`}
                      >
                        Axis: {a.name} (code {a.code})
                      </button>
                    )
                  })}
                  {d.buttons?.map((b) => {
                    const sel =
                      selected?.deviceId === d.id &&
                      selected?.code === b.code &&
                      selected?.eventType === 1
                    return (
                      <button
                        key={b.code}
                        onClick={() =>
                          setSelected({ deviceId: d.id, code: b.code, eventType: 1 })
                        }
                        className={`w-full text-left px-3 py-1 rounded text-xs transition-colors ${
                          sel
                            ? 'bg-brand-500 text-white'
                            : 'hover:bg-gray-100 dark:hover:bg-gray-700'
                        }`}
                      >
                        Button: {b.name} (code {b.code})
                      </button>
                    )
                  })}
                </div>
              </div>
            ))
          )}
        </div>

        {/* Right: Channel slots */}
        <div className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700 overflow-y-auto max-h-[70vh]">
          <h2 className="text-sm font-semibold mb-3 text-gray-600 dark:text-gray-400">
            RC Channels — {selected ? 'click channel to assign' : 'select a source first'}
          </h2>
          <div className="space-y-2">
            {Array.from({ length: NUM_CHANNELS }, (_, i) => {
              const rule = getRule(i)
              return (
                <div
                  key={i}
                  className="border border-gray-100 dark:border-gray-700 rounded-lg p-2"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-gray-500">CH{i + 1}</span>
                    {rule ? (
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-600 dark:text-gray-400">
                          {rule.event_type === 0 ? 'Axis' : 'Btn'} {rule.code} @ {rule.device_id.split('/').pop()}
                        </span>
                        <button
                          onClick={() => clearChannel(i)}
                          className="text-xs text-red-400 hover:text-red-600"
                        >
                          ✕
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => assignToChannel(i)}
                        disabled={!selected}
                        className="text-xs px-2 py-0.5 rounded bg-gray-100 dark:bg-gray-700 hover:bg-brand-500 hover:text-white disabled:opacity-30 transition-colors"
                      >
                        Assign
                      </button>
                    )}
                  </div>
                  {rule && (
                    <>
                      <TransformEditor
                        transform={rule.transform}
                        onChange={(t) => updateTransform(i, t)}
                      />
                      <div className="flex items-center gap-2 mt-2">
                        <label className="text-xs text-gray-500 shrink-0">Failsafe</label>
                        <input
                          type="number"
                          min="-1"
                          max="1"
                          step="0.01"
                          value={rule.failsafe}
                          onChange={(e) => updateFailsafe(i, parseFloat(e.target.value))}
                          className="border border-gray-300 dark:border-gray-600 rounded px-2 py-1 bg-transparent text-xs w-24 tabular-nums"
                        />
                        <span className="text-xs text-gray-400">−1 to 1</span>
                      </div>
                    </>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
}
