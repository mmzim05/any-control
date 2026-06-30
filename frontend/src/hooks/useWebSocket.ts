import { useEffect, useRef, useState } from 'react'

export type WsMessage = {
  type: string
  payload: unknown
}

export function useWebSocket(onMessage: (msg: WsMessage) => void) {
  const ws = useRef<WebSocket | null>(null)
  const [connected, setConnected] = useState(false)
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage

  useEffect(() => {
    let reconnectTimer: ReturnType<typeof setTimeout>

    function connect() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${protocol}//${window.location.host}/ws`
      const socket = new WebSocket(url)
      ws.current = socket

      socket.onopen = () => setConnected(true)
      socket.onclose = () => {
        setConnected(false)
        reconnectTimer = setTimeout(connect, 2000)
      }
      socket.onerror = () => socket.close()
      socket.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data) as WsMessage
          onMessageRef.current(msg)
        } catch {}
      }
    }

    connect()
    return () => {
      clearTimeout(reconnectTimer)
      ws.current?.close()
    }
  }, [])

  return connected
}
