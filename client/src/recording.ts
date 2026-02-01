// Recording status from server
export interface RecordingStatus {
  available: boolean;
  recording: boolean;
  filePath?: string;
  startTime?: number;
  durationMs?: number;
  bytesWritten?: number;
  framesWritten?: number;
}

// Recording file info for listings
export interface RecordingFile {
  filename: string;
  sizeBytes: number;
  createdAt: number;
  durationMs: number;
}

// Get current recording status
export async function getRecordingStatus(
  cameraIndex: number,
): Promise<RecordingStatus> {
  const response = await fetch(`/camera${cameraIndex + 1}/record/status`, {
    method: "GET",
  });
  if (!response.ok) {
    throw new Error(`Failed to get recording status: ${response.statusText}`);
  }
  return response.json();
}

// Start recording
export async function startRecording(
  cameraIndex: number,
): Promise<RecordingStatus> {
  const response = await fetch(`/camera${cameraIndex + 1}/record/start`, {
    method: "POST",
  });
  if (!response.ok) {
    const error = await response.text();
    throw new Error(
      error || `Failed to start recording: ${response.statusText}`,
    );
  }
  return response.json();
}

// Stop recording
export async function stopRecording(
  cameraIndex: number,
): Promise<RecordingStatus> {
  const response = await fetch(`/camera${cameraIndex + 1}/record/stop`, {
    method: "POST",
  });
  if (!response.ok) {
    const error = await response.text();
    throw new Error(
      error || `Failed to stop recording: ${response.statusText}`,
    );
  }
  return response.json();
}

// List all recordings
export async function listRecordings(
  cameraIndex: number,
): Promise<RecordingFile[]> {
  const response = await fetch(`/camera${cameraIndex + 1}/record/list`, {
    method: "GET",
  });
  if (!response.ok) {
    throw new Error(`Failed to list recordings: ${response.statusText}`);
  }
  const data = await response.json();
  return data.recordings || [];
}

// Get download URL for a recording
export function getDownloadUrl(cameraIndex: number, filename: string): string {
  return `/camera${cameraIndex + 1}/record/download/${encodeURIComponent(filename)}`;
}

// Format duration for display (MM:SS or HH:MM:SS)
export function formatDuration(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, "0")}:${seconds.toString().padStart(2, "0")}`;
  }
  return `${minutes.toString().padStart(2, "0")}:${seconds.toString().padStart(2, "0")}`;
}

// Format bytes for display (KB, MB, GB)
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024)
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

// Format timestamp for display (Jan 31, 14:30)
export function formatDate(timestamp: number): string {
  const date = new Date(timestamp);
  const months = [
    "Jan",
    "Feb",
    "Mar",
    "Apr",
    "May",
    "Jun",
    "Jul",
    "Aug",
    "Sep",
    "Oct",
    "Nov",
    "Dec",
  ];
  const month = months[date.getMonth()];
  const day = date.getDate();
  const hours = date.getHours().toString().padStart(2, "0");
  const minutes = date.getMinutes().toString().padStart(2, "0");
  return `${month} ${day}, ${hours}:${minutes}`;
}

// Parse recording filename to extract date (recording_20260131_143052.h264)
export function parseRecordingDate(filename: string): Date | null {
  const match = filename.match(
    /recording_(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})(\d{2})\.h264/,
  );
  if (!match) return null;

  const [, year, month, day, hour, minute, second] = match;
  return new Date(
    parseInt(year, 10),
    parseInt(month, 10) - 1,
    parseInt(day, 10),
    parseInt(hour, 10),
    parseInt(minute, 10),
    parseInt(second, 10),
  );
}
