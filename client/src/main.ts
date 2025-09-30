import "./style.css";

import { getCameraCount } from "./cameras";
import { startStream } from "./connect";

const cameraCount = await getCameraCount();
console.log("Found", cameraCount, "cameras");
const container = document.querySelector<HTMLDivElement>(".container")!;
// Set grid for tiling
if (cameraCount > 0) {
  const cols = Math.ceil(Math.sqrt(cameraCount));
  const rows = Math.ceil(cameraCount / cols);
  container.style.gridTemplateColumns = `repeat(${cols}, 1fr)`;
  container.style.gridTemplateRows = `repeat(${rows}, 1fr)`;
}
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
