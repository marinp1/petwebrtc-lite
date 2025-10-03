import "./style.css";

import { getCameraCount } from "./cameras";
import { startStream } from "./connect";
import { getStorage, setStorage } from "./storage";

const cameraCount = await getCameraCount();
console.log("Found", cameraCount, "cameras");

const container = document.querySelector<HTMLDivElement>(".container")!;
const recognisitionButton = document.getElementById("recognisition-button")!;

{
  const recognisitionActive = getStorage().recognisitionActive;
  recognisitionButton.style.display = "block";
  recognisitionButton.innerText = `Recognize objects: ${recognisitionActive ? "enabled" : "disabled"}`;
}

recognisitionButton.onclick = (ev) => {
  ev.preventDefault();
  setStorage({ recognisitionActive: !getStorage().recognisitionActive });
  window.location.reload();
};

// Create video elements and start streams
for (let i = 1; i <= cameraCount; i++) {
  const videoElement = document.createElement("video");
  videoElement.autoplay = true;
  videoElement.playsInline = true;
  videoElement.controls = true;
  videoElement.title = `Camera ${i}`;

  const statusContainer = document.createElement("div");
  statusContainer.className = "status";
  const connectionElement = document.createElement("span");
  const dataElement = document.createElement("span");
  statusContainer.append(connectionElement, dataElement);

  const videoContainer = document.createElement("details");
  if (i === 1) videoContainer.open = true;
  const summary = document.createElement("summary");
  summary.innerText = `Camera ${i}`;
  videoContainer.className = "videoContainer";
  videoContainer.appendChild(summary);
  videoContainer.appendChild(videoElement);
  videoContainer.appendChild(statusContainer);
  container.appendChild(videoContainer);

  const url = `/camera${i}`;
  startStream({
    url,
    name: `Camera ${i}`,
    videoElement,
    connectionElement,
    dataElement,
  });
}
