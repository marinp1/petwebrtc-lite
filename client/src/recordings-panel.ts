import {
  formatBytes,
  formatDate,
  formatDuration,
  getDownloadUrl,
  listRecordings,
  parseRecordingDate,
  type RecordingFile,
} from "./recording";

export class RecordingsPanel {
  private dialog: HTMLDialogElement;
  private listContainer: HTMLDivElement;
  private refreshInterval: number | null = null;

  constructor() {
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
      const recordings = await listRecordings();
      this.renderList(recordings);
    } catch (err) {
      console.error("Failed to fetch recordings:", err);
      this.listContainer.innerHTML = `<p class="error">Failed to load recordings</p>`;
    }
  }

  private renderList(recordings: RecordingFile[]): void {
    if (recordings.length === 0) {
      this.listContainer.innerHTML = `<p class="empty">No recordings found</p>`;
      return;
    }

    // Sort by creation date, newest first
    recordings.sort((a, b) => b.createdAt - a.createdAt);

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
    for (const recording of recordings) {
      const row = document.createElement("tr");

      // Parse date from filename or use createdAt
      const date =
        parseRecordingDate(recording.filename) || new Date(recording.createdAt);
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
        <td><a href="${getDownloadUrl(recording.filename)}" class="download-link" download>Download</a></td>
      `;

      tbody.appendChild(row);
    }
    table.appendChild(tbody);

    this.listContainer.innerHTML = "";
    this.listContainer.appendChild(table);
  }
}
