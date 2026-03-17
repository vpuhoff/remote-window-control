import "./styles.css";

import { bootstrapAuth, clearToken } from "./auth.js";

if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").catch(() => {});
}
import { attachGestureControls } from "./gestures.js";
import { attachInstallPrompt } from "./install-prompt.js";
import { attachKeyboardBridge } from "./keyboard.js";
import { attachViewportSync } from "./viewport.js";
import { createRemoteConnection } from "./webrtc.js";

const videoElement = document.querySelector("#remote-video");
const videoStageElement = document.querySelector("#video-stage");
const statusElement = document.querySelector("#status-pill");
const bitrateElement = document.querySelector("#bitrate-pill");
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
      [100, 300, 600].forEach((ms) => window.setTimeout(() => onFullscreenChange?.(), ms));
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
      onBitrate: (kbps) => {
        if (bitrateElement) {
          bitrateElement.textContent = `${kbps} kbps`;
          bitrateElement.hidden = false;
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
    attachInstallPrompt(document.body, setStatus);
    setStatus("Управление готово");
  } catch (error) {
    clearToken();
    setStatus(error instanceof Error ? error.message : "Ошибка инициализации");
  }
}

main();
