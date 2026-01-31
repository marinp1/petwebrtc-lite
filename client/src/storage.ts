interface State {
  recognisitionActive: boolean;
  currentCameraIndex: number | null;
}

export const getStorage = (): State => {
  const currentCameraIndexStr =
    window.localStorage.getItem("currentCameraIndex");
  return {
    recognisitionActive:
      window.localStorage.getItem("recognisitionActive") === "true",
    currentCameraIndex:
      currentCameraIndexStr !== null
        ? Number.parseInt(currentCameraIndexStr, 10)
        : null,
  };
};

export const setStorage = (newState: Partial<State>) => {
  for (const [key, val] of Object.entries(newState)) {
    window.localStorage.setItem(key, String(val));
  }
};
