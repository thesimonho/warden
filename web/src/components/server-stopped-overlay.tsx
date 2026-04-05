/**
 * Full-page overlay shown when the Warden server has stopped.
 *
 * Displayed when the SSE connection drops after receiving a
 * `server_shutdown` event, indicating an intentional shutdown
 * (not a network error or crash). Prevents the user from
 * interacting with a dead dashboard.
 */
export default function ServerStoppedOverlay() {
  return (
    <div
      className={[
        // Layout — full viewport overlay
        'fixed inset-0 z-50 flex flex-col items-center justify-center gap-16',
        // Appearance
        'bg-background/95 backdrop-blur-sm',
      ].join(' ')}
    >
      <img src="/logo.svg" alt="Warden" className="h-8 opacity-40 dark:invert" />
      <div className="flex flex-col items-center gap-4">
        <h2 className="text-foreground text-xl font-semibold">Warden has stopped</h2>
        <p className="text-muted-foreground max-w-sm text-center text-sm">
          The server has been shut down. Running containers are unaffected.
          <br />
          Restart Warden to reconnect.
        </p>
      </div>
    </div>
  )
}
