import "./styles.css";

import { bootstrapAuth, clearToken } from "./auth.js";
import { attachGestureControls } from "./gestures.js";
import { attachKeyboardBridge } from "./keyboard.js";
import { attachViewportSync } from "./viewport.js";
import { createRemoteConnection } from "./webrtc.js";

const videoElement = document.querySelector("#remote-video");
const videoStageElement = document.querySelector("#video-stage");
const statusElement = document.querySelector("#status-pill");
const fullscreenButton = document.querySelector("#fullscreen-button");
const keyboardButton = document.querySelector("#keyboard-button");
const hiddenInput = document.querySelector("#hidden-text-input");
const backspaceButton = document.querySelector("#backspace-button");
const enterButton = document.querySelector("#enter-button");

function setStatus(message) {
  statusElement.textContent = message;
}

function attachTopBarControls(onFullscreenChange) {
  fullscreenButton?.addEventListener("click", async () => {
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await document.documentElement.requestFullscreen();
      }
      onFullscreenChange?.();
      window.setTimeout(() => onFullscreenChange?.(), 150);
    } catch {
      setStatus("Не удалось переключить полный экран");
    }
  });
}

async function main() {
  try {
    const token = await bootstrapAuth();
    if (!token) {
      setStatus("Откройте секретную ссылку с параметром secret.");
      return;
    }

    const remote = await createRemoteConnection({
      token,
      videoElement,
      onStatus: setStatus,
      onInputMessage: (payload) => {
        if (payload.type === "host.ack") {
          setStatus(payload.message);
        }
      },
    });

    attachGestureControls(videoElement, remote.sendControl, setStatus);
    const keyboard = attachKeyboardBridge({
      buttonElement: keyboardButton,
      inputElement: hiddenInput,
      backspaceButton,
      enterButton,
    }, remote.sendControl, setStatus);
    const viewportSync = attachViewportSync(remote.sendControl, {
      isSuspended: () => keyboard.isActive(),
      targetElement: videoStageElement,
    });
    attachTopBarControls(viewportSync.triggerFullscreenSync);
    setStatus("Управление готово");
  } catch (error) {
    clearToken();
    setStatus(error instanceof Error ? error.message : "Ошибка инициализации");
  }
}

main();
