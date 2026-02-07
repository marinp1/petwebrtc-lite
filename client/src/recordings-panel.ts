import type { CameraInfo } from "./cameras";
import {
  formatBytes,
  formatDate,
  formatDuration,
  getDownloadUrl,
  listRecordings,
  parseRecordingDate,
  type RecordingFile,
} from "./recording";

interface CameraRecordings {
  camera: CameraInfo;
  available: boolean;
  recordings: RecordingFile[];
  error?: string;
}

export class RecordingsPanel {
  private cameraView: HTMLElement;
  private recordingsView: HTMLElement;
  private listContainer: HTMLDivElement;
  private loadingElement: HTMLElement;
  private backButton: HTMLElement;
  private refreshInterval: number | null = null;
  private cameras: CameraInfo[];
  private isVisible = false;
  private onCloseCallback: (() => void) | null = null;

  constructor(cameras: CameraInfo[]) {
    this.cameras = cameras;
    this.cameraView = document.getElementById("camera-view")!;
    this.recordingsView = document.getElementById("recordings-view")!;
    this.listContainer = this.recordingsView.querySelector(
      ".recordings-list",
    ) as HTMLDivElement;
    this.loadingElement = this.recordingsView.querySelector(
      ".recordings-loading",
    ) as HTMLElement;
    this.backButton = document.getElementById("recordings-back")!;

    // Back button handler
    this.backButton.addEventListener("click", () => this.close());

    // Escape key to close
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && this.isVisible) {
        this.close();
      }
    });
  }

  /**
   * Register a callback to be called when the panel closes
   */
  onClose(callback: () => void): void {
    this.onCloseCallback = callback;
  }

  async open(): Promise<void> {
    if (this.isVisible) return;

    this.isVisible = true;

    // Show loading state
    this.loadingElement.style.display = "block";
    this.listContainer.innerHTML = "";

    // Switch views
    this.cameraView.classList.remove("active");
    this.recordingsView.classList.add("active");

    // Fetch data
    await this.refresh();

    // Hide loading
    this.loadingElement.style.display = "none";

    // Auto-refresh every 5 seconds while open
    this.refreshInterval = window.setInterval(() => {
      this.refresh().catch(console.error);
    }, 5000);
  }

  close(): void {
    if (!this.isVisible) return;

    this.isVisible = false;

    if (this.refreshInterval) {
      clearInterval(this.refreshInterval);
      this.refreshInterval = null;
    }

    // Switch views back
    this.recordingsView.classList.remove("active");
    this.cameraView.classList.add("active");

    // Notify listeners
    if (this.onCloseCallback) {
      this.onCloseCallback();
    }
  }

  async refresh(): Promise<void> {
    try {
      // Fetch recordings from all cameras in parallel
      const cameraRecordings: CameraRecordings[] = await Promise.all(
        this.cameras.map(async (camera) => {
          try {
            const recordings = await listRecordings(camera.endpoint);
            return {
              camera,
              available: true,
              recordings,
            };
          } catch (err) {
            return {
              camera,
              available: false,
              recordings: [],
              error: err instanceof Error ? err.message : String(err),
            };
          }
        }),
      );

      this.renderList(cameraRecordings);
    } catch (err) {
      console.error("Failed to fetch recordings:", err);
      this.listContainer.innerHTML = `<p class="recordings-error">Failed to load recordings</p>`;
    }
  }

  private renderList(cameraRecordings: CameraRecordings[]): void {
    this.listContainer.innerHTML = "";

    // Count total recordings
    const totalRecordings = cameraRecordings.reduce(
      (sum, cam) => sum + cam.recordings.length,
      0,
    );

    // Check if any camera has recording available
    const anyAvailable = cameraRecordings.some((cam) => cam.available);

    if (!anyAvailable) {
      this.listContainer.innerHTML = `<p class="recordings-empty">Recording not available on any camera</p>`;
      return;
    }

    if (totalRecordings === 0) {
      this.listContainer.innerHTML = `<p class="recordings-empty">No recordings found</p>`;
      return;
    }

    // Render recordings grouped by camera
    for (const camData of cameraRecordings) {
      if (!camData.available) {
        // Show unavailable camera
        const section = document.createElement("section");
        section.className = "camera-section";
        section.innerHTML = `
          <h3 class="camera-section-title">${camData.camera.title}</h3>
          <p class="camera-unavailable">Recording not available</p>
        `;
        this.listContainer.appendChild(section);
        continue;
      }

      if (camData.recordings.length === 0) {
        // Show camera with no recordings
        const section = document.createElement("section");
        section.className = "camera-section";
        section.innerHTML = `
          <h3 class="camera-section-title">${camData.camera.title}</h3>
          <p class="camera-empty">No recordings</p>
        `;
        this.listContainer.appendChild(section);
        continue;
      }

      // Show camera with recordings
      const section = document.createElement("section");
      section.className = "camera-section";

      const header = document.createElement("h3");
      header.className = "camera-section-title";
      header.textContent = `${camData.camera.title} (${camData.recordings.length})`;
      section.appendChild(header);

      // Sort by creation date, newest first
      const sortedRecordings = [...camData.recordings].sort(
        (a, b) => b.createdAt - a.createdAt,
      );

      // Create recording cards
      const cardList = document.createElement("div");
      cardList.className = "recording-cards";

      for (const recording of sortedRecordings) {
        const card = document.createElement("a");
        card.className = "recording-card";
        card.href = getDownloadUrl(camData.camera.endpoint, recording.filename);
        card.download = recording.filename;

        // Parse date from filename or use createdAt
        const date =
          parseRecordingDate(recording.filename) ||
          new Date(recording.createdAt);
        const dateStr = formatDate(date.getTime());

        // Duration
        const durationStr =
          recording.durationMs > 0 ? formatDuration(recording.durationMs) : "-";

        // Size
        const sizeStr = formatBytes(recording.sizeBytes);

        card.innerHTML = `
          <div class="recording-card-main">
            <span class="recording-date">${dateStr}</span>
            <span class="recording-duration">${durationStr}</span>
          </div>
          <div class="recording-card-meta">
            <span class="recording-size">${sizeStr}</span>
            <span class="recording-download">Download</span>
          </div>
        `;

        cardList.appendChild(card);
      }

      section.appendChild(cardList);
      this.listContainer.appendChild(section);
    }
  }
}
