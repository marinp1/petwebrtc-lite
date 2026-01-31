import { startObjectDetection } from "./detector";
import { getStorage } from "./storage";

/**
 * Start a WebRTC stream from the given video feed configuration.
 * Sets up a RTCPeerConnection, handles ICE candidates, and manages the video element.
 *
 * Production-grade notes in-code:
 * - Use structured messages on the stats data channel (JSON).
 * - Rely on connectionState events for authoritative connection time.
 * - Update visible badges directly (no DOM observation required).
 *
 * @returns RTCPeerConnection instance for lifecycle management (or null on error)
 */
export async function startStream(videoFeedConfig: {
  url: string;
  name?: string;
  videoElement: HTMLVideoElement;
  connectionElement: HTMLElement;
  // visible badge elements
  droppedElement?: HTMLElement;
  timeElement?: HTMLElement;
}): Promise<RTCPeerConnection | null> {
  const { url, videoElement, connectionElement, droppedElement, timeElement } =
    videoFeedConfig;
  try {
    const pc = new RTCPeerConnection({
      iceServers: [],
    });

    pc.addTransceiver("video", { direction: "recvonly" });

    // Create data channel as offerer for stats (structured JSON)
    const dataChannel = pc.createDataChannel("stats");
    console.log("Created data channel:", dataChannel.label);

    // Timer state for "time connected" badge
    let timerId: number | null = null;
    let connectedSince: number | null = null;

    const formatTime = (s: number) => {
      const mm = Math.floor(s / 60)
        .toString()
        .padStart(2, "0");
      const ss = Math.floor(s % 60)
        .toString()
        .padStart(2, "0");
      return `${mm}:${ss}`;
    };

    const startTimer = () => {
      if (!timeElement) return;
      if (timerId) return;
      connectedSince = Date.now();
      timeElement.textContent = "00:00";
      timerId = window.setInterval(() => {
        const secs = Math.max(
          0,
          Math.floor((Date.now() - (connectedSince ?? Date.now())) / 1000),
        );
        timeElement.textContent = formatTime(secs);
      }, 1000);
    };

    const stopTimer = (reset = true) => {
      if (!timeElement) return;
      if (timerId) {
        clearInterval(timerId);
        timerId = null;
      }
      connectedSince = null;
      if (reset) timeElement.textContent = "00:00";
    };

    dataChannel.onopen = () => {
      console.log("Data channel is open");
    };

    dataChannel.onmessage = (event) => {
      // Expect JSON: { sentFrames: number, droppedFrames: number, timestamp: number }
      try {
        const stats = JSON.parse(event.data);
        if (typeof stats.droppedFrames === "number" && droppedElement) {
          // use the numeric droppedFrames value directly (no regex/parsing)
          droppedElement.textContent = String(stats.droppedFrames);
        }
      } catch (e) {
        console.error(`Error parsing stats:`, e);
      }
    };

    dataChannel.onerror = (error) => {
      console.error(`Data channel error:`, error);
    };

    dataChannel.onclose = () => {
      console.log(`Data channel closed`);
    };

    pc.onicecandidate = (event) => {
      console.log("ICE candidate:", event.candidate);
    };

    pc.ontrack = (event) => {
      console.log("Received remote track", event);
      if (event.streams?.[0]) {
        videoElement.srcObject = event.streams[0];
        videoElement.onloadedmetadata = () => {
          videoElement.play();
          if (getStorage().recognisitionActive) {
            startObjectDetection(videoElement);
          }
        };
      } else {
        console.warn("No streams in ontrack event", event);
      }
      // mark connected optimistically here; authoritative change handled below
      connectionElement.setAttribute("data-status", "connected");
    };

    // Use connection state changes to control authoritative UI and timer
    pc.onconnectionstatechange = () => {
      console.log("Connection state:", pc.connectionState);
      const state = pc.connectionState;
      if (state === "connected") {
        connectionElement.setAttribute("data-status", "connected");
        startTimer();
      } else if (
        state === "failed" ||
        state === "disconnected" ||
        state === "closed"
      ) {
        connectionElement.setAttribute("data-status", "disconnected");
        stopTimer(true);
      } else {
        // other states (checking, connecting) -> show disconnected icon
        connectionElement.setAttribute("data-status", "disconnected");
      }
    };

    pc.oniceconnectionstatechange = () => {
      console.log("ICE connection state:", pc.iceConnectionState);
    };

    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    await Promise.race<void>([
      new Promise((resolve) => {
        if (pc.iceGatheringState === "complete") {
          resolve();
        } else {
          pc.addEventListener(
            "icegatheringstatechange",
            function onStateChange() {
              if (pc.iceGatheringState === "complete") {
                pc.removeEventListener(
                  "icegatheringstatechange",
                  onStateChange,
                );
                resolve();
              }
            },
          );
        }
      }),
      new Promise((resolve) => setTimeout(resolve, 2000)),
    ]);

    console.log("Offer SDP (ICE complete):\n", pc.localDescription?.sdp);

    const res = await fetch(`${url}/offer`, {
      method: "POST",
      body: JSON.stringify(pc.localDescription),
      headers: { "Content-Type": "application/json" },
    });

    if (!res.ok) {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    }

    const answer = await res.json();
    console.log("Received answer from server:", answer);
    await pc.setRemoteDescription(answer);
    console.log("Set remote description");

    // Return the peer connection for lifecycle management
    return pc;
  } catch (err) {
    console.error("Error:", err);
    if (err instanceof Error) {
      connectionElement.textContent = `Error: ${err.message}`;
    } else {
      connectionElement.textContent = `Error: ${String(err)}`;
    }
    return null;
  }
}
