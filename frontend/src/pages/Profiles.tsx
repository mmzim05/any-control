import { useEffect, useState } from 'react'
import type { Profile } from '../hooks/useSimLink'
import { apiFetch } from '../hooks/useSimLink'

export default function Profiles() {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [status, setStatus] = useState('')

  function flash(msg: string) {
    setStatus(msg)
    setTimeout(() => setStatus(''), 2500)
  }

  useEffect(() => {
    load()
  }, [])

  async function load() {
    try {
      const data = await apiFetch<Profile[]>('/api/profiles')
      setProfiles(data)
    } catch {}
  }

  async function createProfile() {
    if (!newName.trim()) return
    try {
      await apiFetch('/api/profiles', {
        method: 'POST',
        body: JSON.stringify({ name: newName, description: newDesc }),
      })
      setCreating(false)
      setNewName('')
      setNewDesc('')
      load()
      flash('Created')
    } catch {
      flash('Create failed')
    }
  }

  async function activate(id: number) {
    try {
      await apiFetch(`/api/profiles/${id}/activate`, { method: 'POST' })
      flash('Profile loaded')
    } catch {
      flash('Load failed')
    }
  }

  async function deleteProfile(id: number) {
    if (!confirm('Delete this profile?')) return
    try {
      await apiFetch(`/api/profiles/${id}`, { method: 'DELETE' })
      load()
      flash('Deleted')
    } catch {
      flash('Delete failed')
    }
  }

  async function exportProfile(p: Profile) {
    const blob = new Blob([JSON.stringify(p, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `simlink-profile-${p.name.replace(/\s+/g, '-')}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  async function importProfile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const text = await file.text()
      const data = JSON.parse(text)
      await apiFetch('/api/profiles', {
        method: 'POST',
        body: JSON.stringify({ name: data.name + ' (import)', description: data.description, config: data.config }),
      })
      load()
      flash('Imported')
    } catch {
      flash('Import failed')
    }
    e.target.value = ''
  }

  return (
    <div className="max-w-3xl mx-auto space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Profiles</h1>
        <div className="flex items-center gap-2">
          {status && <span className="text-sm text-green-600 dark:text-green-400">{status}</span>}
          <label className="px-3 py-1.5 border border-gray-300 dark:border-gray-600 rounded-lg text-sm cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700">
            Import
            <input type="file" accept=".json" className="hidden" onChange={importProfile} />
          </label>
          <button
            onClick={() => setCreating(true)}
            className="px-4 py-1.5 bg-brand-500 hover:bg-brand-600 text-white rounded-lg text-sm font-medium"
          >
            + New
          </button>
        </div>
      </div>

      {creating && (
        <div className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700 space-y-3">
          <h2 className="text-sm font-semibold">New Profile</h2>
          <input
            type="text"
            placeholder="Name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-transparent text-sm"
            autoFocus
          />
          <input
            type="text"
            placeholder="Description (optional)"
            value={newDesc}
            onChange={(e) => setNewDesc(e.target.value)}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 bg-transparent text-sm"
          />
          <div className="flex gap-2">
            <button
              onClick={createProfile}
              className="px-4 py-1.5 bg-brand-500 hover:bg-brand-600 text-white rounded-lg text-sm"
            >
              Create
            </button>
            <button
              onClick={() => setCreating(false)}
              className="px-4 py-1.5 border border-gray-300 dark:border-gray-600 rounded-lg text-sm"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {profiles.length === 0 && !creating && (
        <p className="text-sm text-gray-400 text-center py-12">No profiles yet — create one to save your mapping configuration.</p>
      )}

      <div className="grid gap-3">
        {profiles.map((p) => (
          <div
            key={p.id}
            className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700 flex items-center gap-4"
          >
            <div className="flex-1 min-w-0">
              <p className="font-medium text-sm truncate">{p.name}</p>
              {p.description && (
                <p className="text-xs text-gray-500 mt-0.5 truncate">{p.description}</p>
              )}
              <p className="text-xs text-gray-400 mt-0.5">
                Updated {new Date(p.updated_at).toLocaleDateString()}
              </p>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <button
                onClick={() => activate(p.id)}
                className="px-3 py-1 bg-green-500 hover:bg-green-600 text-white rounded-lg text-xs font-medium"
              >
                Load
              </button>
              <button
                onClick={() => exportProfile(p)}
                className="px-3 py-1 border border-gray-300 dark:border-gray-600 rounded-lg text-xs hover:bg-gray-50 dark:hover:bg-gray-700"
              >
                Export
              </button>
              <button
                onClick={() => deleteProfile(p.id)}
                className="px-3 py-1 border border-red-200 dark:border-red-900 text-red-500 rounded-lg text-xs hover:bg-red-50 dark:hover:bg-red-950"
              >
                Delete
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
