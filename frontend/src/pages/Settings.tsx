import { useEffect, useState } from 'react'
import type { } from '../hooks/useSimLink'
import { apiFetch } from '../hooks/useSimLink'

type TelemetryConfig = {
  enabled: boolean
  interval_ms: number
}

export default function Settings() {
  const [hasPassword, setHasPassword] = useState(false)
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [telemetry, setTelemetry] = useState<TelemetryConfig>({ enabled: false, interval_ms: 100 })
  const [rowCount, setRowCount] = useState<number | null>(null)
  const [status, setStatus] = useState('')

  function flash(msg: string) {
    setStatus(msg)
    setTimeout(() => setStatus(''), 3000)
  }

  useEffect(() => {
    apiFetch<{ has_password: boolean }>('/api/settings').then((s) => setHasPassword(s.has_password)).catch(() => {})
    apiFetch<TelemetryConfig>('/api/telemetry/config').then(setTelemetry).catch(() => {})
    fetchRowCount()
  }, [])

  async function fetchRowCount() {
    try {
      const data = await apiFetch<{ count: number }>('/api/telemetry/count')
      setRowCount(data.count)
    } catch {}
  }

  async function savePassword() {
    if (password !== confirmPassword) {
      flash('Passwords do not match')
      return
    }
    try {
      await apiFetch('/api/settings/password', {
        method: 'PUT',
        body: JSON.stringify({ password }),
      })
      setHasPassword(password.length > 0)
      setPassword('')
      setConfirmPassword('')
      flash(password.length > 0 ? 'Password set' : 'Password cleared')
    } catch {
      flash('Failed to save password')
    }
  }

  async function saveTelemetry() {
    try {
      const saved = await apiFetch<TelemetryConfig>('/api/telemetry/config', {
        method: 'PUT',
        body: JSON.stringify(telemetry),
      })
      setTelemetry(saved)
      flash('Telemetry settings saved')
    } catch {
      flash('Save failed')
    }
  }

  async function clearTelemetry() {
    if (!confirm('Delete all telemetry data?')) return
    try {
      await apiFetch('/api/telemetry', { method: 'DELETE' })
      setRowCount(0)
      flash('Telemetry cleared')
    } catch {
      flash('Clear failed')
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Settings</h1>
        {status && <span className="text-sm text-green-600 dark:text-green-400">{status}</span>}
      </div>

      {/* Auth section */}
      <div className="bg-white dark:bg-gray-800 rounded-xl p-5 shadow-sm border border-gray-100 dark:border-gray-700 space-y-4">
        <h2 className="text-sm font-semibold text-gray-600 dark:text-gray-400">UI Password</h2>
        <p className="text-xs text-gray-500">
          {hasPassword ? 'Password is set. Enter a new password to change, or leave blank to disable.' : 'No password set — UI is open to anyone on the network.'}
        </p>
        <div className="space-y-2">
          <input
            type="password"
            placeholder={hasPassword ? 'New password (blank to disable)' : 'Set password'}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-transparent text-sm"
          />
          <input
            type="password"
            placeholder="Confirm password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-transparent text-sm"
          />
        </div>
        <button
          onClick={savePassword}
          className="px-4 py-1.5 bg-brand-500 hover:bg-brand-600 text-white rounded-lg text-sm font-medium"
        >
          {hasPassword && password === '' ? 'Clear Password' : 'Save Password'}
        </button>
      </div>

      {/* Telemetry section */}
      <div className="bg-white dark:bg-gray-800 rounded-xl p-5 shadow-sm border border-gray-100 dark:border-gray-700 space-y-4">
        <h2 className="text-sm font-semibold text-gray-600 dark:text-gray-400">Telemetry Logging</h2>

        <div className="flex items-center gap-3">
          <input
            type="checkbox"
            id="telem-enabled"
            checked={telemetry.enabled}
            onChange={(e) => setTelemetry({ ...telemetry, enabled: e.target.checked })}
          />
          <label htmlFor="telem-enabled" className="text-sm">Enable logging</label>
        </div>

        <div>
          <label className="block text-sm mb-1">Log interval</label>
          <div className="flex items-center gap-2">
            <input
              type="number"
              min="50"
              max="5000"
              step="50"
              value={telemetry.interval_ms}
              onChange={(e) => setTelemetry({ ...telemetry, interval_ms: parseInt(e.target.value) || 100 })}
              className="border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-transparent text-sm w-32"
            />
            <span className="text-sm text-gray-500">ms</span>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={saveTelemetry}
            className="px-4 py-1.5 bg-brand-500 hover:bg-brand-600 text-white rounded-lg text-sm font-medium"
          >
            Save
          </button>
          <a
            href="/api/telemetry/export"
            className="px-4 py-1.5 border border-gray-300 dark:border-gray-600 rounded-lg text-sm hover:bg-gray-50 dark:hover:bg-gray-700"
            download="telemetry.csv"
          >
            Export CSV
          </a>
          <button
            onClick={clearTelemetry}
            className="px-4 py-1.5 border border-red-200 dark:border-red-900 text-red-500 rounded-lg text-sm hover:bg-red-50 dark:hover:bg-red-950"
          >
            Clear All
          </button>
        </div>

        {rowCount !== null && (
          <p className="text-xs text-gray-400">{rowCount.toLocaleString()} rows stored (max 100,000)</p>
        )}
      </div>
    </div>
  )
}
