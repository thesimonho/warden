import { useEffect, useRef } from 'react'

/**
 * Runs a callback on a fixed interval, cleaning up automatically.
 * Pass `null` as `delayMs` to pause without unmounting.
 *
 * Uses a ref to hold the callback so the interval is only reset when
 * `delayMs` changes — filter or dependency changes in the callback
 * do not restart the countdown.
 *
 * @param callback - The function to call on each tick.
 * @param delayMs - Interval in milliseconds, or `null` to pause.
 */
export function useInterval(callback: () => void, delayMs: number | null): void {
  const savedCallback = useRef(callback)

  useEffect(() => {
    savedCallback.current = callback
  }, [callback])

  useEffect(() => {
    if (delayMs === null) return
    const id = setInterval(() => savedCallback.current(), delayMs)
    return () => clearInterval(id)
  }, [delayMs])
}
