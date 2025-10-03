import type { VideoFeedConfig } from "./types";

/**
 * Start a WebRTC stream from the given video feed configuration.
 * Sets up a RTCPeerConnection, handles ICE candidates, and manages the video element.
 */
export async function startStream(videoFeedConfig: VideoFeedConfig) {
  const { url, videoElement, connectionElement, dataElement } = videoFeedConfig;
  try {
    const pc = new RTCPeerConnection({
      iceServers: [],
    });

    // Add a video transceiver to trigger ICE gathering and SDP media section
    pc.addTransceiver("video", { direction: "recvonly" });

    // CREATE DATA CHANNEL ON CLIENT SIDE BEFORE CREATING OFFER
    // This is crucial - the client (offerer) must create the data channel
    const dataChannel = pc.createDataChannel("stats");
    console.log("Created data channel:", dataChannel.label);

    dataChannel.onopen = () => {
      console.log("Data channel is open");
    };

    dataChannel.onmessage = (event) => {
      try {
        const stats: {
          sentFrames: number;
          droppedFrames: number;
          timestamp: number;
        } = JSON.parse(event.data);
        const localeDateTime = new Date(stats.timestamp).toLocaleTimeString();
        const statsText = `${localeDateTime}, sent/dropped: ${stats.sentFrames} / ${stats.droppedFrames}`;
        dataElement.innerText = statsText;
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

    // Log all ICE candidates
    pc.onicecandidate = (event) => {
      console.log("ICE candidate:", event.candidate);
    };

    pc.ontrack = (event) => {
      console.log("Received remote track", event);
      if (event.streams?.[0]) {
        videoElement.srcObject = event.streams[0];
        videoElement.onloadedmetadata = () => {
          videoElement.play();
        };
      } else {
        console.warn("No streams in ontrack event", event);
      }
      connectionElement.textContent = "Connected!";
    };

    // Monitor connection state
    pc.onconnectionstatechange = () => {
      console.log("Connection state:", pc.connectionState);
      connectionElement.textContent = `Connection: ${pc.connectionState}`;
    };

    pc.oniceconnectionstatechange = () => {
      console.log("ICE connection state:", pc.iceConnectionState);
    };

    // Log SDP offer
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    // Wait for ICE gathering to complete before sending offer (with timeout)
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
      new Promise((resolve) => setTimeout(resolve, 2000)), // 2 second timeout
    ]);

    // IMPORTANT: Use pc.localDescription here, after ICE gathering
    console.log("Offer SDP (ICE complete):\n", pc.localDescription?.sdp);

    // Send offer to server
    console.log("Sending offer to server");
    const res = await fetch(`${url}/offer`, {
      method: "POST",
      body: JSON.stringify(pc.localDescription), // use latest localDescription here
      headers: { "Content-Type": "application/json" },
    });

    if (!res.ok) {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    }

    const answer = await res.json();
    console.log("Received answer from server:", answer);
    console.log("Answer SDP:\n", answer.sdp);
    await pc.setRemoteDescription(answer);
    console.log("Set remote description");
  } catch (err) {
    console.error("Error:", err);
    if (err instanceof Error) {
      connectionElement.textContent = `Error: ${err.message}`;
    }
    connectionElement.textContent = `Error: ${err}`;
  }
}
