import { useEffect, useState } from 'react'
import { BrowserRouter, NavLink, Route, Routes } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import Login from './pages/Login'
import Mapping from './pages/Mapping'
import Output from './pages/Output'
import Profiles from './pages/Profiles'
import Settings from './pages/Settings'

function ThemeToggle() {
  const [dark, setDark] = useState(() => {
    const stored = localStorage.getItem('theme')
    if (stored) return stored === 'dark'
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark)
    localStorage.setItem('theme', dark ? 'dark' : 'light')
  }, [dark])

  return (
    <button
      onClick={() => setDark((d) => !d)}
      className="p-2 rounded-lg text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white"
      title="Toggle theme"
    >
      {dark ? '☀️' : '🌙'}
    </button>
  )
}

const navItems = [
  { to: '/', label: 'Dashboard' },
  { to: '/mapping', label: 'Mapping' },
  { to: '/output', label: 'Output' },
  { to: '/profiles', label: 'Profiles' },
  { to: '/settings', label: 'Settings' },
]

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex flex-col">
      <nav className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-2 flex items-center gap-4">
        <span className="font-bold text-brand-500 text-lg mr-4">SimLink</span>
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              `px-3 py-1 rounded-md text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-brand-500 text-white'
                  : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white'
              }`
            }
          >
            {item.label}
          </NavLink>
        ))}
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </nav>
      <main className="flex-1 p-4">{children}</main>
    </div>
  )
}

export default function App() {
  const [authed, setAuthed] = useState<boolean | null>(null)

  useEffect(() => {
    fetch('/api/settings')
      .then((r) => {
        if (r.status === 401) setAuthed(false)
        else { setAuthed(true) }
      })
      .catch(() => setAuthed(true))
  }, [])

  if (authed === null) return null
  if (authed === false) return <Login onLogin={() => setAuthed(true)} />

  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/mapping" element={<Mapping />} />
          <Route path="/output" element={<Output />} />
          <Route path="/profiles" element={<Profiles />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}
