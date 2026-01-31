import "./style.css";

import { getCameraCount } from "./cameras";
import { Carousel } from "./carousel";
import { SwipeGestureHandler } from "./gestures";
import { NavigationUI } from "./navigation";
import { getStorage, setStorage } from "./storage";

const cameraCount = await getCameraCount();
console.log("Found", cameraCount, "cameras");

const container = document.querySelector<HTMLDivElement>(".container")!;
const navContainer = document.querySelector<HTMLDivElement>(".carousel-nav")!;
const recognisitionButton = document.getElementById("recognisition-button")!;

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
  slideContainer.appendChild(statusContainer);
  container.appendChild(slideContainer);

  // Register camera with carousel
  carousel.registerCamera(i, {
    videoElement,
    container: slideContainer,
    connectionElement,
    droppedElement,
    timeElement,
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
