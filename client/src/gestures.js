const LONG_PRESS_MS = 450;

function normalizePoint(event, element) {
  const rect = element.getBoundingClientRect();
  const touch = event.touches[0] ?? event.changedTouches[0];
  const x = (touch.clientX - rect.left) / rect.width;
  const y = (touch.clientY - rect.top) / rect.height;

  return {
    x: Math.min(1, Math.max(0, x)),
    y: Math.min(1, Math.max(0, y)),
  };
}

export function attachGestureControls(videoElement, sendControl, onStatus) {
  let longPressTimer = null;
  let dragging = false;
  let previousTwoFingerCenter = null;

  const cancelLongPress = () => {
    if (longPressTimer) {
      window.clearTimeout(longPressTimer);
      longPressTimer = null;
    }
  };

  videoElement.addEventListener("touchstart", (event) => {
    event.preventDefault();

    if (event.touches.length === 1) {
      const point = normalizePoint(event, videoElement);
      dragging = false;
      longPressTimer = window.setTimeout(() => {
        dragging = true;
        sendControl({
          type: "input.mouseDown",
          button: "right",
          x: point.x,
          y: point.y,
        });
        onStatus("Long press активирован");
      }, LONG_PRESS_MS);
      return;
    }

    if (event.touches.length === 2) {
      cancelLongPress();
      const [a, b] = event.touches;
      previousTwoFingerCenter = {
        x: (a.clientX + b.clientX) / 2,
        y: (a.clientY + b.clientY) / 2,
      };
    }
  }, { passive: false });

  videoElement.addEventListener("touchmove", (event) => {
    event.preventDefault();

    if (event.touches.length === 1 && dragging) {
      const point = normalizePoint(event, videoElement);
      sendControl({
        type: "input.mouseMove",
        x: point.x,
        y: point.y,
      });
      return;
    }

    if (event.touches.length === 2 && previousTwoFingerCenter) {
      const [a, b] = event.touches;
      const currentCenter = {
        x: (a.clientX + b.clientX) / 2,
        y: (a.clientY + b.clientY) / 2,
      };

      sendControl({
        type: "input.scroll",
        deltaX: currentCenter.x - previousTwoFingerCenter.x,
        deltaY: currentCenter.y - previousTwoFingerCenter.y,
      });
      previousTwoFingerCenter = currentCenter;
    }
  }, { passive: false });

  videoElement.addEventListener("touchend", (event) => {
    event.preventDefault();
    const point = normalizePoint(event, videoElement);

    if (dragging) {
      sendControl({
        type: "input.mouseUp",
        button: "right",
        x: point.x,
        y: point.y,
      });
    } else if (event.changedTouches.length === 1) {
      sendControl({
        type: "input.tap",
        button: "left",
        x: point.x,
        y: point.y,
      });
    }

    dragging = false;
    previousTwoFingerCenter = null;
    cancelLongPress();
  }, { passive: false });

  videoElement.addEventListener("touchcancel", () => {
    dragging = false;
    previousTwoFingerCenter = null;
    cancelLongPress();
  });
}
