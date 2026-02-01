import "./style.css";

import { getCameraCount } from "./cameras";
import { Carousel } from "./carousel";
import { SwipeGestureHandler } from "./gestures";
import { NavigationUI } from "./navigation";
import {
  formatDuration,
  getRecordingStatus,
  startRecording,
  stopRecording,
} from "./recording";
import { RecordingsPanel } from "./recordings-panel";
import { getStorage, setStorage } from "./storage";

const cameraCount = await getCameraCount();
console.log("Found", cameraCount, "cameras");

const container = document.querySelector<HTMLDivElement>(".container")!;
const navContainer = document.querySelector<HTMLDivElement>(".carousel-nav")!;
const recognisitionButton = document.getElementById("recognisition-button")!;
const recordButton = document.getElementById(
  "record-button",
) as HTMLButtonElement | null;
const recordingsButton = document.getElementById(
  "recordings-button",
) as HTMLButtonElement | null;

// Initialize carousel
const carousel = new Carousel(cameraCount, container);

// Apply semantic button state & text for detection toggle
{
  const recognisitionActive = getStorage().recognisitionActive;
  recognisitionButton.classList.remove("btn-ghost");
  recognisitionButton.classList.add("btn");
  (recognisitionButton as HTMLButtonElement).ariaPressed =
    String(recognisitionActive);
  recognisitionButton.innerText = `Recognize objects: ${recognisitionActive ? "enabled" : "disabled"}`;
}

recognisitionButton.onclick = (ev) => {
  ev.preventDefault();
  setStorage({ recognisitionActive: !getStorage().recognisitionActive });
  window.location.reload();
};

// Create video elements and register with carousel
for (let i = 0; i < cameraCount; i++) {
  const videoElement = document.createElement("video");
  videoElement.autoplay = true;
  videoElement.playsInline = true;
  videoElement.controls = true;
  videoElement.title = `Camera ${i + 1}`;
  videoElement.className = "camera-video";

  // Connection status banner (for prominent state display)
  const connectionBanner = document.createElement("div");
  connectionBanner.className = "connection-banner";
  connectionBanner.setAttribute("data-status", "disconnected");

  const bannerIcon = document.createElement("span");
  bannerIcon.className = "banner-icon";

  const bannerText = document.createElement("span");
  bannerText.className = "banner-text";

  const reconnectButton = document.createElement("button");
  reconnectButton.className = "reconnect-button";
  reconnectButton.textContent = "Reconnect";
  reconnectButton.setAttribute("aria-label", "Reconnect to camera");
  reconnectButton.onclick = () => {
    console.log(`Reconnect button clicked for camera ${i + 1}`);
    carousel.reconnect(i).catch((err) => {
      console.error(`Manual reconnect failed for camera ${i + 1}:`, err);
    });
  };

  connectionBanner.append(bannerIcon, bannerText, reconnectButton);

  // Stats overlay (compact display when connected)
  const statsOverlay = document.createElement("div");
  statsOverlay.className = "stats-overlay";

  const healthIndicator = document.createElement("span");
  healthIndicator.className = "health-indicator";
  healthIndicator.setAttribute("data-health", "healthy");

  const uptimeDisplay = document.createElement("span");
  uptimeDisplay.className = "uptime-display";
  uptimeDisplay.textContent = "00:00";

  const droppedDisplay = document.createElement("span");
  droppedDisplay.className = "dropped-display";
  droppedDisplay.textContent = "0";
  droppedDisplay.setAttribute("data-health", "healthy");

  statsOverlay.append(healthIndicator, uptimeDisplay, droppedDisplay);

  // Carousel slide container
  const slideContainer = document.createElement("div");
  slideContainer.className = "carousel-slide card";

  // title overlay (visible on the video)
  const titleElement = document.createElement("span");
  titleElement.className = "badge title";
  titleElement.textContent = `Camera ${i + 1}`;
  titleElement.setAttribute("aria-hidden", "true");

  // media wrapper to control aspect ratio
  const media = document.createElement("div");
  media.className = "media";
  media.appendChild(videoElement);

  slideContainer.appendChild(media);
  slideContainer.appendChild(titleElement);
  slideContainer.appendChild(connectionBanner);
  slideContainer.appendChild(statsOverlay);
  container.appendChild(slideContainer);

  // Register camera with carousel
  carousel.registerCamera(i, {
    videoElement,
    container: slideContainer,
    connectionElement: connectionBanner,
    droppedElement: droppedDisplay,
    timeElement: uptimeDisplay,
  });
}

// Initialize gesture handler (instance used for side effects)
void new SwipeGestureHandler(container, carousel);

// Initialize navigation UI (instance used for side effects)
void new NavigationUI(carousel, navContainer);

// Initialize carousel connections (connect to current + preload adjacent)
carousel.initialize().catch((err) => {
  console.error("Failed to initialize carousel:", err);
});

// Recording functionality
let statusPollingId: number | null = null;
let recordingsPanel: RecordingsPanel | null = null;

function updateRecordButton(
  recording: boolean,
  finalizing: boolean = false,
): void {
  if (!recordButton) return;
  recordButton.ariaPressed = String(recording || finalizing);
  recordButton.classList.toggle("recording", recording);
  recordButton.disabled = finalizing; // Disable button during finalization
  const label = recordButton.querySelector(".record-label");
  if (label) {
    if (finalizing) {
      label.textContent = "Finalizing MP4...";
    } else if (!recording) {
      label.textContent = "Record";
    }
  }
}

function startStatusPolling(): void {
  if (statusPollingId) return;

  statusPollingId = window.setInterval(async () => {
    try {
      const currentCameraIndex = carousel.getCurrentIndex();
      const status = await getRecordingStatus(currentCameraIndex);
      updateRecordButton(status.recording, status.finalizing);

      // Update duration display while recording
      if (status.recording && status.durationMs !== undefined) {
        const label = recordButton?.querySelector(".record-label");
        if (label) {
          label.textContent = formatDuration(status.durationMs);
        }
      }

      // Stop polling when recording and finalization are both complete
      if (!status.recording && !status.finalizing) {
        stopStatusPolling();
      }
    } catch (err) {
      console.error("Failed to poll recording status:", err);
    }
  }, 1000);
}

function stopStatusPolling(): void {
  if (statusPollingId) {
    clearInterval(statusPollingId);
    statusPollingId = null;
  }
}

async function initRecording(): Promise<void> {
  if (!recordButton || !recordingsButton) return;
  try {
    // Check recording availability on all cameras
    const availabilityChecks = [];
    for (let i = 0; i < cameraCount; i++) {
      availabilityChecks.push(
        getRecordingStatus(i).catch(() => ({
          available: false,
          recording: false,
          finalizing: false,
        })),
      );
    }

    const statuses = await Promise.all(availabilityChecks);
    const availableCameras = statuses.filter((s) => s.available).length;

    if (availableCameras === 0) {
      recordButton.style.display = "none";
      return;
    }

    console.log(`Recording available on ${availableCameras} camera(s)`);

    // Show the buttons
    recordButton.hidden = false;
    recordingsButton.hidden = false;

    // Function to update recording button for current camera
    const updateForCurrentCamera = async () => {
      const currentCameraIndex = carousel.getCurrentIndex();
      const status = await getRecordingStatus(currentCameraIndex).catch(() => ({
        available: false,
        recording: false,
        finalizing: false,
      }));

      // Hide/show button based on current camera's support
      recordButton.style.display = status.available ? "" : "none";

      if (status.available) {
        updateRecordButton(status.recording, status.finalizing);
        if (status.recording || status.finalizing) {
          startStatusPolling();
        }
      } else {
        stopStatusPolling();
      }
    };

    // Update for initial camera
    await updateForCurrentCamera();

    // Record button click handler
    recordButton.onclick = async (ev) => {
      ev.preventDefault();
      recordButton.disabled = true;

      try {
        const currentCameraIndex = carousel.getCurrentIndex();
        const currentStatus = await getRecordingStatus(currentCameraIndex);

        if (currentStatus.recording) {
          await stopRecording(currentCameraIndex);
          // Don't update button yet - wait for status polling to show finalization
          startStatusPolling(); // Continue polling to show "Finalizing MP4..."
        } else {
          await startRecording(currentCameraIndex);
          updateRecordButton(true);
          startStatusPolling();
        }
      } catch (err) {
        console.error("Recording error:", err);
        alert(`Recording error: ${err instanceof Error ? err.message : err}`);
      } finally {
        recordButton.disabled = false;
      }
    };

    // Initialize recordings panel
    recordingsPanel = new RecordingsPanel(cameraCount);

    // Update when camera changes
    carousel.onIndexChange(async () => {
      await updateForCurrentCamera();
    });

    // Recordings button click handler
    recordingsButton.onclick = async (ev) => {
      ev.preventDefault();
      if (recordingsPanel) {
        await recordingsPanel.open();
      }
    };
  } catch (err) {
    console.error("Failed to initialize recording:", err);
  }
}

// Initialize recording UI
initRecording().catch((err) => {
  console.error("Recording initialization error:", err);
});
