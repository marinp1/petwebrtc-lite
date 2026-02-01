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
  cameraIndex: number;
  available: boolean;
  recordings: RecordingFile[];
  error?: string;
}

export class RecordingsPanel {
  private dialog: HTMLDialogElement;
  private listContainer: HTMLDivElement;
  private refreshInterval: number | null = null;
  private cameraCount: number;

  constructor(cameraCount: number) {
    this.cameraCount = cameraCount;
    this.dialog = document.getElementById(
      "recordings-panel",
    ) as HTMLDialogElement;
    this.listContainer = this.dialog.querySelector(
      ".recordings-list",
    ) as HTMLDivElement;

    // Close button handler
    const closeButton = this.dialog.querySelector(".close-btn");
    if (closeButton) {
      closeButton.addEventListener("click", () => this.close());
    }

    // Close on backdrop click
    this.dialog.addEventListener("click", (e) => {
      if (e.target === this.dialog) {
        this.close();
      }
    });

    // Close on escape key
    this.dialog.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        this.close();
      }
    });
  }

  async open(): Promise<void> {
    this.dialog.showModal();
    await this.refresh();

    // Auto-refresh every 5 seconds while open
    this.refreshInterval = window.setInterval(() => {
      this.refresh().catch(console.error);
    }, 5000);
  }

  close(): void {
    if (this.refreshInterval) {
      clearInterval(this.refreshInterval);
      this.refreshInterval = null;
    }
    this.dialog.close();
  }

  async refresh(): Promise<void> {
    try {
      // Fetch recordings from all cameras in parallel
      const cameraRecordings: CameraRecordings[] = await Promise.all(
        Array.from({ length: this.cameraCount }, async (_, i) => {
          try {
            const recordings = await listRecordings(i);
            return {
              cameraIndex: i,
              available: true,
              recordings,
            };
          } catch (err) {
            return {
              cameraIndex: i,
              available: false,
              recordings: [],
              error: err instanceof Error ? err.message : String(err),
            };
          }
        })
      );

      this.renderList(cameraRecordings);
    } catch (err) {
      console.error("Failed to fetch recordings:", err);
      this.listContainer.innerHTML = `<p class="error">Failed to load recordings</p>`;
    }
  }

  private renderList(cameraRecordings: CameraRecordings[]): void {
    this.listContainer.innerHTML = "";

    // Count total recordings
    const totalRecordings = cameraRecordings.reduce(
      (sum, cam) => sum + cam.recordings.length,
      0
    );

    // Check if any camera has recording available
    const anyAvailable = cameraRecordings.some(cam => cam.available);

    if (!anyAvailable) {
      this.listContainer.innerHTML = `<p class="empty">Recording not available on any camera</p>`;
      return;
    }

    if (totalRecordings === 0) {
      this.listContainer.innerHTML = `<p class="empty">No recordings found</p>`;
      return;
    }

    // Render recordings grouped by camera
    for (const camData of cameraRecordings) {
      if (!camData.available) {
        // Show unavailable camera
        const section = document.createElement("div");
        section.className = "camera-recordings-section";
        section.innerHTML = `
          <h3>Camera ${camData.cameraIndex + 1}</h3>
          <p class="unavailable">Recording not available</p>
        `;
        this.listContainer.appendChild(section);
        continue;
      }

      if (camData.recordings.length === 0) {
        // Show camera with no recordings
        const section = document.createElement("div");
        section.className = "camera-recordings-section";
        section.innerHTML = `
          <h3>Camera ${camData.cameraIndex + 1}</h3>
          <p class="empty-camera">No recordings</p>
        `;
        this.listContainer.appendChild(section);
        continue;
      }

      // Show camera with recordings
      const section = document.createElement("div");
      section.className = "camera-recordings-section";

      const header = document.createElement("h3");
      header.textContent = `Camera ${camData.cameraIndex + 1} (${camData.recordings.length})`;
      section.appendChild(header);

      // Sort by creation date, newest first
      const sortedRecordings = [...camData.recordings].sort(
        (a, b) => b.createdAt - a.createdAt
      );

      const table = document.createElement("table");
      table.className = "recordings-table";

      // Header
      const thead = document.createElement("thead");
      thead.innerHTML = `
        <tr>
          <th>Date</th>
          <th>Duration</th>
          <th>Size</th>
          <th>Download</th>
        </tr>
      `;
      table.appendChild(thead);

      // Body
      const tbody = document.createElement("tbody");
      for (const recording of sortedRecordings) {
        const row = document.createElement("tr");

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

        row.innerHTML = `
          <td>${dateStr}</td>
          <td>${durationStr}</td>
          <td>${sizeStr}</td>
          <td><a href="${getDownloadUrl(camData.cameraIndex, recording.filename)}" class="download-link" download>Download</a></td>
        `;

        tbody.appendChild(row);
      }
      table.appendChild(tbody);

      section.appendChild(table);
      this.listContainer.appendChild(section);
    }
  }
}
