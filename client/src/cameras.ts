export interface CameraInfo {
  endpoint: string;
  title: string;
}

export async function getCameras(): Promise<CameraInfo[]> {
  const response = await fetch("/cameras", { method: "GET" });
  if (!response.ok) {
    throw new Error(`Failed to fetch cameras: ${response.statusText}`);
  }
  return response.json();
}
