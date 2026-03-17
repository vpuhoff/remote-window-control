function getViewportPayload(targetElement) {
  const stageRect = targetElement?.getBoundingClientRect();
  const viewport = window.visualViewport;

  const width = Math.round(stageRect?.width ?? viewport?.width ?? window.innerWidth);
  const height = Math.round(stageRect?.height ?? viewport?.height ?? window.innerHeight);

  return {
    type: "viewport.resize",
    width,
    height,
    devicePixelRatio: window.devicePixelRatio || 1,
  };
}

export function attachViewportSync(sendControl, options = {}) {
  let timerId = null;
  let orientationTimerIds = [];
  const isSuspended = options.isSuspended ?? (() => false);
  const targetElement = options.targetElement ?? null;
  let lastPayload = null;

  const sendViewport = () => {
    const payload = getViewportPayload(targetElement);
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
    lastPayload = null;
    scheduleViewportSync();
    for (const timeoutId of orientationTimerIds) {
      window.clearTimeout(timeoutId);
    }

    orientationTimerIds = [0, 100, 250, 500, 900, 1500].map((delayMs) =>
      window.setTimeout(() => sendViewport(), delayMs)
    );
  };

  const onFullscreenChange = () => {
    lastPayload = null;
    requestAnimationFrame(() => {
      scheduleViewportSync();
      scheduleOrientationSync();
    });
  };

  const triggerFullscreenSync = () => {
    onFullscreenChange();
  };

  const orientationMedia = window.matchMedia?.("(orientation: portrait)");
  const onOrientationChange = () => scheduleOrientationSync();

  sendViewport();
  window.addEventListener("resize", scheduleViewportSync);
  window.addEventListener("orientationchange", onOrientationChange);
  orientationMedia?.addEventListener?.("change", onOrientationChange);
  window.visualViewport?.addEventListener("resize", scheduleViewportSync);
  document.addEventListener("fullscreenchange", onFullscreenChange);
  document.addEventListener("webkitfullscreenchange", onFullscreenChange);
  if (screen.orientation?.addEventListener) {
    screen.orientation.addEventListener("change", onOrientationChange);
  }

  return {
    triggerFullscreenSync,
    cleanup() {
      window.removeEventListener("resize", scheduleViewportSync);
      window.removeEventListener("orientationchange", onOrientationChange);
      orientationMedia?.removeEventListener?.("change", onOrientationChange);
      window.visualViewport?.removeEventListener("resize", scheduleViewportSync);
      document.removeEventListener("fullscreenchange", onFullscreenChange);
      document.removeEventListener("webkitfullscreenchange", onFullscreenChange);
      screen.orientation?.removeEventListener?.("change", onOrientationChange);
      if (timerId) {
        window.clearTimeout(timerId);
      }
      for (const timeoutId of orientationTimerIds) {
        window.clearTimeout(timeoutId);
      }
    },
  };
}
