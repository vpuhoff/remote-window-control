import "./styles.css";

import { bootstrapAuth, clearToken } from "./auth.js";
import { attachGestureControls } from "./gestures.js";
import { attachKeyboardBridge } from "./keyboard.js";
import { attachViewportSync } from "./viewport.js";
import { createRemoteConnection } from "./webrtc.js";

const videoElement = document.querySelector("#remote-video");
const statusElement = document.querySelector("#status-pill");
const keyboardButton = document.querySelector("#keyboard-button");
const hiddenInput = document.querySelector("#hidden-text-input");

function setStatus(message) {
  statusElement.textContent = message;
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
    attachKeyboardBridge(keyboardButton, hiddenInput, remote.sendControl);
    attachViewportSync(remote.sendControl);
    setStatus("Управление готово");
  } catch (error) {
    clearToken();
    setStatus(error instanceof Error ? error.message : "Ошибка инициализации");
  }
}

main();
