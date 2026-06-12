// Server-Sent Events subscription with automatic reconnect.
// Event shape mirrors src/events.go: {type, instance_id, ts, data}.

export interface ServerEvent {
  type: "snapshot" | "session" | "players" | "server" | "content_job" | "drivers" | "positions";
  instance_id: number;
  ts: number;
  data: any;
}

export type EventHandler = (event: ServerEvent) => void;

export function subscribeServerEvents(onEvent: EventHandler, onStateChange?: (connected: boolean) => void): () => void {
  let source: EventSource | null = null;
  let stopped = false;
  let retryDelay = 1000;

  const connect = () => {
    if (stopped) return;
    source = new EventSource("/api/server/events");

    source.onopen = () => {
      retryDelay = 1000;
      onStateChange?.(true);
    };

    source.onmessage = (msg) => {
      try {
        onEvent(JSON.parse(msg.data));
      } catch {
        // malformed frame — ignore
      }
    };

    source.onerror = () => {
      onStateChange?.(false);
      source?.close();
      source = null;
      if (!stopped) {
        setTimeout(connect, retryDelay);
        retryDelay = Math.min(retryDelay * 2, 15000);
      }
    };
  };

  connect();

  return () => {
    stopped = true;
    source?.close();
    source = null;
  };
}
