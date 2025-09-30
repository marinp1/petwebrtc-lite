import "./style.css";

import { getCameraCount } from "./cameras";
import { startStream } from "./connect";

const cameraCount = await getCameraCount();
console.log("Found", cameraCount, "cameras");

const container = document.querySelector<HTMLDivElement>(".container")!;

// Function to update layout
function updateLayout() {
  if (cameraCount === 1) {
    container.style.gridTemplateColumns = "1fr";
    container.style.gridTemplateRows = "1fr";
  } else if (cameraCount === 2) {
    if (window.innerWidth >= window.innerHeight) {
      // Landscape → side by side
      container.style.gridTemplateColumns = "repeat(2, 1fr)";
      container.style.gridTemplateRows = "1fr";
    } else {
      // Portrait → stacked vertically
      container.style.gridTemplateColumns = "1fr";
      container.style.gridTemplateRows = "repeat(2, 1fr)";
    }
  }
}

// Initial layout + on resize
updateLayout();
window.addEventListener("resize", updateLayout);

// Create video elements and start streams
for (let i = 1; i <= cameraCount; i++) {
  const videoElement = document.createElement("video");
  videoElement.autoplay = true;
  videoElement.playsInline = true;
  videoElement.controls = true;
  videoElement.title = `Camera ${i}`;

  const statusElement = document.createElement("span");
  statusElement.className = "status";

  const videoContainer = document.createElement("div");
  videoContainer.className = "videoContainer";
  videoContainer.appendChild(videoElement);
  videoContainer.appendChild(statusElement);
  container.appendChild(videoContainer);

  const url = `/camera${i}`;
  startStream({ url, name: `Camera ${i}`, videoElement, statusElement });
}
