const checkCameraStatus = async (url: string) => {
  const abortController = new AbortController();
  try {
    const response = await Promise.race<{ ok: boolean }>([
      fetch(url, { method: "GET", signal: abortController.signal }),
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error("Timeout")), 1000),
      ),
    ]);
    return response.ok;
  } catch (error) {
    abortController.abort();
    console.debug("Error checking camera status:", error);
    return false;
  }
};

export async function getCameraCount() {
  let cameraCount = 0;
  while (await checkCameraStatus(`/camera${cameraCount + 1}/status`)) {
    cameraCount++;
  }
  return cameraCount;
}
