interface State {
  recognisitionActive: boolean;
}

export const getStorage = (): State => {
  return {
    recognisitionActive:
      window.localStorage.getItem("recognisitionActive") === "true",
  };
};

export const setStorage = (newState: Partial<State>) => {
  for (const [key, val] of Object.entries(newState)) {
    window.localStorage.setItem(key, String(val));
  }
};
