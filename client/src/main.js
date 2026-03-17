import "./styles.css";

import { bootstrapAuth, clearToken, getStoredToken } from "./auth.js";
import { fetchWindows, setTargetWindow } from "./api.js";

if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").catch(() => {});
}
import { attachGestureControls } from "./gestures.js";
import { attachInstallPrompt } from "./install-prompt.js";
import { attachKeyboardBridge } from "./keyboard.js";
import { attachViewportSync } from "./viewport.js";
import { createRemoteConnection } from "./webrtc.js";

const selectScreen = document.querySelector("#select-screen");
const remoteScreen = document.querySelector("#remote-screen");
const selectStatus = document.querySelector("#select-status");
const windowList = document.querySelector("#window-list");
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

function escapeHtml(str) {
  return String(str)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

async function showWindowSelect(token) {
  try {
    if (selectStatus) selectStatus.textContent = "Загрузка списка окон...";
    const windows = await fetchWindows(token);
    console.log("Загружено окон:", windows.length, windows);
    if (windows.length === 0) {
      if (selectStatus) selectStatus.textContent = "Нет доступных окон";
      return;
    }
    if (selectStatus) selectStatus.textContent = "";
    if (windowList) windowList.innerHTML = "";
    for (const w of windows) {
      const card = document.createElement("button");
      card.type = "button";
      card.className = "window-card";
      const title = w.title || "(без названия)";
      const sub = w.process_name ? ` · ${w.process_name}` : "";
      card.innerHTML = `
        <span class="window-card-title">${escapeHtml(title)}</span>
        <span class="window-card-sub">${escapeHtml(sub)}</span>
      `;
      card.addEventListener("click", async () => {
        card.disabled = true;
        if (selectStatus) selectStatus.textContent = "Подключение...";
        try {
          await setTargetWindow(token, w.handle);
          if (selectScreen) selectScreen.hidden = true;
          if (remoteScreen) remoteScreen.hidden = false;
          await startRemoteControl(getStoredToken());
        } catch (err) {
          console.error("Ошибка подключения:", err);
          if (selectScreen) selectScreen.hidden = false;
          if (remoteScreen) remoteScreen.hidden = true;
          const msg = err instanceof Error ? err.message : "Ошибка";
          if (selectStatus) selectStatus.textContent = msg;
          if (statusElement) statusElement.textContent = msg;
          card.disabled = false;
        }
      });
      windowList?.appendChild(card);
    }
  } catch (err) {
    if (selectStatus) selectStatus.textContent = err instanceof Error ? err.message : "Ошибка загрузки";
  }
}

async function startRemoteControl(token) {
  try {
    const remote = await createRemoteConnection({
      token,
      videoElement,
      onStatus: setStatus,
      onInputMessage: (payload) => {
        if (payload.type === "host.ack") setStatus(payload.message);
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
    setStatus("Управление готово");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : "Ошибка подключения");
  }
}

async function main() {
  attachInstallPrompt(document.body, (msg) => {
    if (selectStatus) selectStatus.textContent = msg;
    if (statusElement) statusElement.textContent = msg;
  });

  try {
    const token = await bootstrapAuth();
    if (!token) {
      if (selectStatus) selectStatus.textContent = "Откройте секретную ссылку с параметром secret.";
      if (statusElement) statusElement.textContent = "Откройте секретную ссылку с параметром secret.";
      return;
    }

    await showWindowSelect(token);
  } catch (error) {
    clearToken();
    if (selectStatus) selectStatus.textContent = error instanceof Error ? error.message : "Ошибка";
  }
}

main();
