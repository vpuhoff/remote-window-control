const LONG_PRESS_MS = 450;
const TAP_SLOP_PX = 12;
const SCROLL_SENSITIVITY = 0.8;

function getVideoContentRect(element) {
  const rect = element.getBoundingClientRect();
  const sourceWidth = element.videoWidth;
  const sourceHeight = element.videoHeight;

  if (!sourceWidth || !sourceHeight || !rect.width || !rect.height) {
    return rect;
  }

  const scale = Math.min(rect.width / sourceWidth, rect.height / sourceHeight);
  const contentWidth = sourceWidth * scale;
  const contentHeight = sourceHeight * scale;

  return {
    left: rect.left + (rect.width - contentWidth) / 2,
    top: rect.top + (rect.height - contentHeight) / 2,
    width: contentWidth,
    height: contentHeight,
  };
}

function normalizePoint(event, element) {
  const rect = getVideoContentRect(element);
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
  let oneFingerScrolling = false;
  let lastScrollPoint = null;
  let lastTapPoint = null;
  let previousTwoFingerCenter = null;
  let touchStartPoint = null;
  let longPressPoint = null;
  let tapCancelled = false;

  const distance = (a, b) => {
    if (!a || !b) {
      return 0;
    }

    return Math.hypot(a.x - b.x, a.y - b.y);
  };

  const firstTouchClientPoint = (event) => {
    const touch = event.touches[0] ?? event.changedTouches[0];
    if (!touch) {
      return null;
    }

    return { x: touch.clientX, y: touch.clientY };
  };

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
      touchStartPoint = firstTouchClientPoint(event);
      longPressPoint = point;
      tapCancelled = false;
      dragging = false;
      longPressTimer = window.setTimeout(() => {
        if (tapCancelled) {
          return;
        }

        dragging = true;
        sendControl({
          type: "input.mouseDown",
          button: "right",
          x: longPressPoint.x,
          y: longPressPoint.y,
        });
        onStatus("Long press активирован");
      }, LONG_PRESS_MS);
      return;
    }

    if (event.touches.length === 2) {
      cancelLongPress();
      tapCancelled = true;
      const [a, b] = event.touches;
      previousTwoFingerCenter = {
        x: (a.clientX + b.clientX) / 2,
        y: (a.clientY + b.clientY) / 2,
      };
    }
  }, { passive: false });

  videoElement.addEventListener("touchmove", (event) => {
    event.preventDefault();

    if (event.touches.length === 1) {
      longPressPoint = normalizePoint(event, videoElement);
      const currentTouchPoint = firstTouchClientPoint(event);
      if (!dragging && distance(touchStartPoint, currentTouchPoint) > TAP_SLOP_PX) {
        tapCancelled = true;
        cancelLongPress();
        if (!oneFingerScrolling) {
          oneFingerScrolling = true;
          lastScrollPoint = currentTouchPoint;
        }
      }
      if (oneFingerScrolling && lastScrollPoint) {
        const deltaX = (currentTouchPoint.x - lastScrollPoint.x) * SCROLL_SENSITIVITY;
        const deltaY = (lastScrollPoint.y - currentTouchPoint.y) * SCROLL_SENSITIVITY;
        const scrollPos = lastTapPoint ?? longPressPoint;
        sendControl({ type: "input.scroll", deltaX, deltaY, x: scrollPos.x, y: scrollPos.y });
        lastScrollPoint = currentTouchPoint;
        return;
      }
    }

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

      const scrollPos = lastTapPoint ?? { x: 0.5, y: 0.5 };
      sendControl({
        type: "input.scroll",
        deltaX: currentCenter.x - previousTwoFingerCenter.x,
        deltaY: currentCenter.y - previousTwoFingerCenter.y,
        x: scrollPos.x,
        y: scrollPos.y,
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
    } else if (!tapCancelled && !oneFingerScrolling && event.changedTouches.length === 1) {
      lastTapPoint = point;
      sendControl({
        type: "input.tap",
        button: "left",
        x: point.x,
        y: point.y,
      });
    }

    dragging = false;
    oneFingerScrolling = false;
    lastScrollPoint = null;
    previousTwoFingerCenter = null;
    touchStartPoint = null;
    longPressPoint = null;
    tapCancelled = false;
    cancelLongPress();
  }, { passive: false });

  videoElement.addEventListener("touchcancel", () => {
    dragging = false;
    oneFingerScrolling = false;
    lastScrollPoint = null;
    previousTwoFingerCenter = null;
    touchStartPoint = null;
    longPressPoint = null;
    tapCancelled = false;
    cancelLongPress();
  });
}
