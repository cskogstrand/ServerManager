// Minimal WHEP (WebRTC-HTTP Egress Protocol) client for pulling a receive-only
// stream from MediaMTX (or any WHEP server) into a <video> element.
//
// Handshake (non-trickle, single round-trip):
//   1. createOffer + setLocalDescription, then wait for ICE gathering so the
//      offer SDP already carries every candidate (no PATCH/trickle needed).
//   2. POST the offer SDP to the WHEP endpoint (Content-Type: application/sdp).
//   3. Server replies 201 Created with the answer SDP in the body and a
//      `Location` header pointing at the per-session resource URL.
//   4. setRemoteDescription(answer) → media flows.
// Teardown: DELETE the resource URL so the server frees the reader slot.
//
// MediaMTX exposes this at  http(s)://<host>/<path>/whep  (WebRTC must be
// enabled in mediamtx.yml and port 8889 reverse-proxied with TLS — browsers
// require https for the page hosting WebRTC).
import { ref } from "vue";

export type WhepState = "idle" | "connecting" | "playing" | "error";

export function useWhep() {
  const state = ref<WhepState>("idle");
  const error = ref<string | null>(null);

  let pc: RTCPeerConnection | null = null;
  let resourceUrl: string | null = null;
  let abort: AbortController | null = null;

  async function connect(video: HTMLVideoElement, endpoint: string) {
    await disconnect();
    state.value = "connecting";
    error.value = null;
    abort = new AbortController();

    // Tracks arrive after negotiation; attach them to a stream the <video> owns.
    const stream = new MediaStream();
    video.srcObject = stream;

    pc = new RTCPeerConnection({
      iceServers: [{ urls: "stun:stun.l.google.com:19302" }],
    });
    // Receive-only: we pull, we never send. Reserve one slot for each kind so
    // the offer advertises both even before the server's tracks are known.
    pc.addTransceiver("video", { direction: "recvonly" });
    pc.addTransceiver("audio", { direction: "recvonly" });

    pc.ontrack = (e) => {
      stream.addTrack(e.track);
      void video.play().catch(() => {}); // autoplay; muted keeps it allowed
    };
    pc.onconnectionstatechange = () => {
      const s = pc?.connectionState;
      if (s === "connected") state.value = "playing";
      else if (s === "failed" || s === "disconnected" || s === "closed") {
        if (state.value !== "idle") {
          state.value = "error";
          error.value = `peer connection ${s}`;
        }
      }
    };

    try {
      await pc.setLocalDescription(await pc.createOffer());
      await waitIceGathering(pc);

      const res = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/sdp" },
        body: pc.localDescription!.sdp,
        signal: abort.signal,
      });
      if (res.status !== 201) {
        // Non-201 isn't an exception — handle it the same way the catch does
        // (set error, free resources) without throwing only to catch locally.
        state.value = "error";
        error.value = `WHEP POST returned ${res.status}`;
        await disconnect();
        return;
      }

      const loc = res.headers.get("Location");
      if (loc) resourceUrl = new URL(loc, endpoint).toString();

      const answer = await res.text();
      await pc.setRemoteDescription({ type: "answer", sdp: answer });
    } catch (e) {
      if ((e as Error).name === "AbortError") return; // disconnect() raced us
      state.value = "error";
      error.value = (e as Error).message || "stream connection failed";
      await disconnect();
    }
  }

  async function disconnect() {
    abort?.abort();
    abort = null;
    if (resourceUrl) {
      // Best-effort: free the server-side reader. Don't await — page may be
      // unmounting and we don't care about the response.
      void fetch(resourceUrl, { method: "DELETE" }).catch(() => {});
      resourceUrl = null;
    }
    pc?.getReceivers().forEach((r) => r.track?.stop());
    pc?.close();
    pc = null;
    state.value = "idle";
  }

  return { state, error, connect, disconnect };
}

// Resolve once ICE gathering completes, or after a short cap so a stuck
// candidate (e.g. blocked STUN) can't hang the whole connect forever.
function waitIceGathering(pc: RTCPeerConnection, timeoutMs = 2000): Promise<void> {
  if (pc.iceGatheringState === "complete") return Promise.resolve();
  return new Promise((resolve) => {
    const finish = () => {
      pc.removeEventListener("icegatheringstatechange", check);
      resolve();
    };
    const check = () => {
      if (pc.iceGatheringState === "complete") finish();
    };
    pc.addEventListener("icegatheringstatechange", check);
    setTimeout(finish, timeoutMs);
  });
}
