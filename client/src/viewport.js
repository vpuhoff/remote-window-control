function getViewportPayload() {
  const viewport = window.visualViewport;
  return {
    type: "viewport.resize",
    width: Math.round(viewport?.width ?? window.innerWidth),
    height: Math.round(viewport?.height ?? window.innerHeight),
    devicePixelRatio: window.devicePixelRatio || 1,
  };
}

export function attachViewportSync(sendControl, options = {}) {
  let timerId = null;
  let orientationTimerIds = [];
  const isSuspended = options.isSuspended ?? (() => false);
  let lastPayload = null;

  const sendViewport = () => {
    const payload = getViewportPayload();
    const keyboardLikeResize =
      isSuspended()
      && lastPayload
      && payload.width === lastPayload.width
      && payload.height !== lastPayload.height;

    if (keyboardLikeResize) {
      return;
    }

    if (
      lastPayload
      && payload.width === lastPayload.width
      && payload.height === lastPayload.height
      && payload.devicePixelRatio === lastPayload.devicePixelRatio
    ) {
      return;
    }

    lastPayload = payload;
    sendControl(payload);
  };

  const scheduleViewportSync = () => {
    if (timerId) {
      window.clearTimeout(timerId);
    }

    timerId = window.setTimeout(sendViewport, 120);
  };

  const scheduleOrientationSync = () => {
    scheduleViewportSync();
    for (const timeoutId of orientationTimerIds) {
      window.clearTimeout(timeoutId);
    }

    orientationTimerIds = [250, 500, 900].map((delayMs) => (
      window.setTimeout(sendViewport, delayMs)
    ));
  };

  sendViewport();
  window.addEventListener("resize", scheduleViewportSync);
  window.addEventListener("orientationchange", scheduleOrientationSync);
  window.visualViewport?.addEventListener("resize", scheduleViewportSync);

  return () => {
    window.removeEventListener("resize", scheduleViewportSync);
    window.removeEventListener("orientationchange", scheduleOrientationSync);
    window.visualViewport?.removeEventListener("resize", scheduleViewportSync);
    if (timerId) {
      window.clearTimeout(timerId);
    }
    for (const timeoutId of orientationTimerIds) {
      window.clearTimeout(timeoutId);
    }
  };
}
