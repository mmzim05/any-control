import { useState } from 'react'
import { useWebSocket } from '../hooks/useWebSocket'
import type { Device } from '../hooks/useSimLink'

const NUM_CHANNELS = 32

function ChannelBar({ index, value }: { index: number; value: number }) {
  // value is [-1, 1]; display as 0–100% with center at 50%
  const pct = ((value + 1) / 2) * 100
  return (
    <div className="flex items-center gap-2">
      <span className="text-xs w-6 text-right text-gray-500">{index + 1}</span>
      <div className="flex-1 h-4 bg-gray-200 dark:bg-gray-700 rounded overflow-hidden relative">
        {/* Center line */}
        <div className="absolute top-0 bottom-0 left-1/2 w-px bg-gray-400 dark:bg-gray-500" />
        {/* Fill from center */}
        {value >= 0 ? (
          <div
            className="absolute top-0 bottom-0 bg-brand-500"
            style={{ left: '50%', width: `${pct - 50}%` }}
          />
        ) : (
          <div
            className="absolute top-0 bottom-0 bg-orange-400"
            style={{ left: `${pct}%`, width: `${50 - pct}%` }}
          />
        )}
      </div>
      <span className="text-xs w-12 text-right tabular-nums text-gray-600 dark:text-gray-400">
        {value.toFixed(2)}
      </span>
    </div>
  )
}

export default function Dashboard() {
  const [channels, setChannels] = useState<number[]>(new Array(NUM_CHANNELS).fill(0))
  const [devices, setDevices] = useState<Device[]>([])
  const connected = useWebSocket((msg) => {
    if (msg.type === 'channels') setChannels(msg.payload as number[])
    if (msg.type === 'devices') setDevices(msg.payload as Device[])
  })

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div className="flex items-center gap-3">
        <h1 className="text-xl font-semibold">Dashboard</h1>
        <span
          className={`text-xs px-2 py-0.5 rounded-full font-medium ${
            connected
              ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
              : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
          }`}
        >
          {connected ? 'Live' : 'Disconnected'}
        </span>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Channel visualizer */}
        <div className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700">
          <h2 className="text-sm font-semibold mb-3 text-gray-700 dark:text-gray-300">
            RC Channels
          </h2>
          <div className="space-y-1">
            {channels.map((v, i) => (
              <ChannelBar key={i} index={i} value={v} />
            ))}
          </div>
        </div>

        {/* Device list */}
        <div className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700">
          <h2 className="text-sm font-semibold mb-3 text-gray-700 dark:text-gray-300">
            Connected Devices ({devices.length})
          </h2>
          {devices.length === 0 ? (
            <p className="text-sm text-gray-400">No sim gear detected</p>
          ) : (
            <ul className="space-y-3">
              {devices.map((d) => (
                <li key={d.id} className="border border-gray-100 dark:border-gray-700 rounded-lg p-3">
                  <p className="font-medium text-sm">{d.name || 'Unknown Device'}</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                    {d.axes?.length ?? 0} axes · {d.buttons?.length ?? 0} buttons
                  </p>
                  <p className="text-xs text-gray-400 font-mono mt-0.5">
                    {d.id} · VID:{d.vendor.toString(16).padStart(4, '0')} PID:
                    {d.product.toString(16).padStart(4, '0')}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
