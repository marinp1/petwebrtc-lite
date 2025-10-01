import "./style.css";

import { getCameraCount } from "./cameras";
import { startStream } from "./connect";

const cameraCount = await getCameraCount();
console.log("Found", cameraCount, "cameras");

const container = document.querySelector<HTMLDivElement>(".container")!;

// Create video elements and start streams
for (let i = 1; i <= cameraCount; i++) {
  const videoElement = document.createElement("video");
  videoElement.autoplay = true;
  videoElement.playsInline = true;
  videoElement.controls = true;
  videoElement.title = `Camera ${i}`;

  const statusElement = document.createElement("span");
  statusElement.className = "status";

  const videoContainer = document.createElement("details");
  if (i === 1) videoContainer.open = true;
  const summary = document.createElement("summary");
  summary.innerText = `Camera ${i}`;
  videoContainer.className = "videoContainer";
  videoContainer.appendChild(summary);
  videoContainer.appendChild(videoElement);
  videoContainer.appendChild(statusElement);
  container.appendChild(videoContainer);

  const url = `/camera${i}`;
  startStream({ url, name: `Camera ${i}`, videoElement, statusElement });
}
