import "./style.css";

import { getCameraCount } from "./cameras";
import { startStream } from "./connect";
import { getStorage, setStorage } from "./storage";

const cameraCount = await getCameraCount();
console.log("Found", cameraCount, "cameras");

const container = document.querySelector<HTMLDivElement>(".container")!;
const recognisitionButton = document.getElementById("recognisition-button")!;

// mark container with camera count class so CSS can adapt layout
container.classList.add(`cols-${Math.min(cameraCount, 4)}`);

// apply semantic button state & text
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

// Create video elements and start streams
for (let i = 1; i <= cameraCount; i++) {
  const videoElement = document.createElement("video");
  videoElement.autoplay = true;
  videoElement.playsInline = true;
  videoElement.controls = true;
  videoElement.title = `Camera ${i}`;
  videoElement.className = "camera-video";

  const statusContainer = document.createElement("div");
  statusContainer.className = "status badges";

  // connection badge (icon only; text hidden by CSS)
  const connectionElement = document.createElement("span");
  connectionElement.className = "badge connection";
  connectionElement.setAttribute("data-status", "disconnected");
  connectionElement.textContent = ""; // visual is the dot, text not needed

  // dropped frames badge (number only)
  const droppedElement = document.createElement("span");
  droppedElement.className = "badge dropped";
  droppedElement.textContent = "0";

  // time connected badge (MM:SS)
  const timeElement = document.createElement("span");
  timeElement.className = "badge time";
  timeElement.textContent = "00:00";

  statusContainer.append(connectionElement, droppedElement, timeElement);

  // use a plain container instead of <details> so clicking the header doesn't hide content
  const videoContainer = document.createElement("div");
  videoContainer.className = "videoContainer card";
  const summary = document.createElement("button");
  // keep a11y label, but render as an overlay button (no visible text)
  summary.className = "videoHeader";
  summary.setAttribute("aria-label", `Camera ${i}`);
  summary.type = "button";
  summary.textContent = "";
  // toggles an 'expanded' class instead of collapsing the element
  summary.addEventListener("click", (ev) => {
    ev.preventDefault();
    videoContainer.classList.toggle("expanded");
  });

  // title overlay (visible on the video)
  const titleElement = document.createElement("span");
  titleElement.className = "badge title";
  titleElement.textContent = `Camera ${i}`;
  titleElement.setAttribute("aria-hidden", "true");

  // media wrapper to control aspect ratio and avoid card growing excessively
  const media = document.createElement("div");
  media.className = "media";
  media.appendChild(videoElement);

  videoContainer.appendChild(summary);
  videoContainer.appendChild(media);
  videoContainer.appendChild(titleElement);
  videoContainer.appendChild(statusContainer);
  container.appendChild(videoContainer);

  const url = `/camera${i}`;
  startStream({
    url,
    name: `Camera ${i}`,
    videoElement,
    connectionElement,
    droppedElement,
    timeElement,
  });
}
