import { startStream } from "./connect";
import { getStorage, setStorage } from "./storage";

interface VideoElements {
  videoElement: HTMLVideoElement;
  container: HTMLElement;
  connectionElement: HTMLElement;
  droppedElement: HTMLElement;
  timeElement: HTMLElement;
}

export class Carousel {
  private currentIndex: number = 0;
  private cameraCount: number;
  private isTransitioning: boolean = false;
  private videoElements: Map<number, VideoElements> = new Map();
  private connections: Map<number, RTCPeerConnection> = new Map();
  private onIndexChangeCallbacks: Array<(index: number) => void> = [];

  constructor(cameraCount: number, _container: HTMLElement) {
    this.cameraCount = cameraCount;

    // Restore last viewed camera from localStorage
    const stored = getStorage().currentCameraIndex;
    if (stored !== null && stored >= 0 && stored < cameraCount) {
      this.currentIndex = stored;
    } else {
      this.currentIndex = 0;
    }
  }

  /**
   * Register a video element and its associated badge elements for a camera
   */
  registerCamera(index: number, elements: VideoElements): void {
    this.videoElements.set(index, elements);

    // Set initial visibility
    if (index === this.currentIndex) {
      elements.container.classList.add("active");
    } else if (index < this.currentIndex) {
      elements.container.classList.add("prev");
    } else {
      elements.container.classList.add("next");
    }
  }

  /**
   * Get the current camera index
   */
  getCurrentIndex(): number {
    return this.currentIndex;
  }

  /**
   * Get total camera count
   */
  getCameraCount(): number {
    return this.cameraCount;
  }

  /**
   * Register a callback to be called when the current index changes
   */
  onIndexChange(callback: (index: number) => void): void {
    this.onIndexChangeCallbacks.push(callback);
  }

  /**
   * Navigate to the next camera
   */
  async next(): Promise<void> {
    if (this.isTransitioning || this.currentIndex >= this.cameraCount - 1) {
      return;
    }
    await this.transitionTo(this.currentIndex + 1);
  }

  /**
   * Navigate to the previous camera
   */
  async previous(): Promise<void> {
    if (this.isTransitioning || this.currentIndex <= 0) {
      return;
    }
    await this.transitionTo(this.currentIndex - 1);
  }

  /**
   * Jump to a specific camera by index
   */
  async jumpTo(index: number): Promise<void> {
    if (
      this.isTransitioning ||
      index < 0 ||
      index >= this.cameraCount ||
      index === this.currentIndex
    ) {
      return;
    }
    await this.transitionTo(index);
  }

  /**
   * Transition to a new camera index
   */
  private async transitionTo(newIndex: number): Promise<void> {
    this.isTransitioning = true;

    const oldIndex = this.currentIndex;
    const oldContainer = this.videoElements.get(oldIndex)?.container;
    const newContainer = this.videoElements.get(newIndex)?.container;

    if (!oldContainer || !newContainer) {
      this.isTransitioning = false;
      return;
    }

    // Ensure connection exists for new camera
    await this.ensureConnection(newIndex);

    // Determine slide direction
    const direction = newIndex > oldIndex ? "left" : "right";

    // Apply transition classes
    if (direction === "left") {
      // Sliding left (next camera)
      newContainer.classList.remove("next");
      newContainer.classList.add("slide-enter-from-right");

      // Force reflow
      void newContainer.offsetWidth;

      newContainer.classList.add("slide-enter-active");
      oldContainer.classList.add("slide-exit-to-left");
    } else {
      // Sliding right (previous camera)
      newContainer.classList.remove("prev");
      newContainer.classList.add("slide-enter-from-left");

      // Force reflow
      void newContainer.offsetWidth;

      newContainer.classList.add("slide-enter-active");
      oldContainer.classList.add("slide-exit-to-right");
    }

    // Wait for animation to complete
    await new Promise((resolve) => setTimeout(resolve, 300));

    // Cleanup old slide classes
    oldContainer.classList.remove(
      "active",
      "slide-exit-to-left",
      "slide-exit-to-right",
    );
    oldContainer.classList.add(newIndex > oldIndex ? "prev" : "next");

    // Set new slide as active
    newContainer.classList.remove(
      "slide-enter-from-left",
      "slide-enter-from-right",
      "slide-enter-active",
    );
    newContainer.classList.add("active");

    // Update current index
    this.currentIndex = newIndex;

    // Persist to localStorage
    setStorage({ currentCameraIndex: newIndex });

    // Notify listeners
    for (const callback of this.onIndexChangeCallbacks) {
      callback(newIndex);
    }

    // Preload adjacent cameras
    this.preloadAdjacent(newIndex);

    // Cleanup distant connections
    this.cleanupDistant(newIndex);

    this.isTransitioning = false;
  }

  /**
   * Ensure a connection exists for the given camera index
   */
  async ensureConnection(index: number): Promise<RTCPeerConnection | null> {
    // Check if connection already exists
    if (this.connections.has(index)) {
      return this.connections.get(index)!;
    }

    // Get video elements
    const elements = this.videoElements.get(index);
    if (!elements) {
      console.error(`No video elements registered for camera ${index}`);
      return null;
    }

    try {
      // Start WebRTC stream
      const pc = await startStream({
        url: `/camera${index + 1}`,
        name: `Camera ${index + 1}`,
        videoElement: elements.videoElement,
        connectionElement: elements.connectionElement,
        droppedElement: elements.droppedElement,
        timeElement: elements.timeElement,
      });

      if (pc) {
        this.connections.set(index, pc);
        console.log(`Connected to camera ${index + 1}`);
      }

      return pc;
    } catch (error) {
      console.error(`Failed to connect to camera ${index + 1}:`, error);
      return null;
    }
  }

  /**
   * Preload connections for adjacent cameras
   */
  private preloadAdjacent(currentIndex: number): void {
    // Preload previous camera
    if (currentIndex > 0) {
      this.ensureConnection(currentIndex - 1).catch((err) => {
        console.warn(`Failed to preload camera ${currentIndex}:`, err);
      });
    }

    // Preload next camera
    if (currentIndex < this.cameraCount - 1) {
      this.ensureConnection(currentIndex + 1).catch((err) => {
        console.warn(`Failed to preload camera ${currentIndex + 2}:`, err);
      });
    }
  }

  /**
   * Cleanup connections that are far from the current camera
   */
  private cleanupDistant(currentIndex: number, threshold = 2): void {
    for (const [index, pc] of this.connections.entries()) {
      if (Math.abs(index - currentIndex) > threshold) {
        console.log(`Cleaning up connection to camera ${index + 1}`);
        pc.close();
        this.connections.delete(index);
      }
    }
  }

  /**
   * Initialize the carousel by connecting to the current camera and preloading adjacent
   */
  async initialize(): Promise<void> {
    // Connect to current camera
    await this.ensureConnection(this.currentIndex);

    // Preload adjacent cameras
    this.preloadAdjacent(this.currentIndex);
  }

  /**
   * Check if we're at the first camera (used for bounce effect)
   */
  isAtStart(): boolean {
    return this.currentIndex === 0;
  }

  /**
   * Check if we're at the last camera (used for bounce effect)
   */
  isAtEnd(): boolean {
    return this.currentIndex === this.cameraCount - 1;
  }
}
