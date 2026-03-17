function getViewportPayload() {
  const viewport = window.visualViewport;
  return {
    type: "viewport.resize",
    width: Math.round(viewport?.width ?? window.innerWidth),
    height: Math.round(viewport?.height ?? window.innerHeight),
    devicePixelRatio: window.devicePixelRatio || 1,
  };
}

export function attachViewportSync(sendControl) {
  let timerId = null;

  const sendViewport = () => {
    sendControl(getViewportPayload());
  };

  const scheduleViewportSync = () => {
    if (timerId) {
      window.clearTimeout(timerId);
    }

    timerId = window.setTimeout(sendViewport, 120);
  };

  sendViewport();
  window.addEventListener("resize", scheduleViewportSync);
  window.addEventListener("orientationchange", scheduleViewportSync);
  window.visualViewport?.addEventListener("resize", scheduleViewportSync);

  return () => {
    window.removeEventListener("resize", scheduleViewportSync);
    window.removeEventListener("orientationchange", scheduleViewportSync);
    window.visualViewport?.removeEventListener("resize", scheduleViewportSync);
    if (timerId) {
      window.clearTimeout(timerId);
    }
  };
}
