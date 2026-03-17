const STORAGE_KEY = "share-app-install-dismissed";

function isStandalone() {
  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    window.navigator.standalone === true ||
    document.referrer.includes("android-app://")
  );
}

function isIOS() {
  return /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
}

export function attachInstallPrompt(container, onStatus) {
  if (isStandalone()) {
    return;
  }

  const banner = document.createElement("div");
  banner.id = "install-banner";
  banner.setAttribute("role", "dialog");
  banner.setAttribute("aria-label", "Установить приложение");
  banner.innerHTML = `
    <div class="install-banner-content">
      <span class="install-banner-text">Установить как приложение</span>
      <div class="install-banner-actions">
        <button type="button" id="install-confirm" class="install-btn install-btn-primary">Установить</button>
        <button type="button" id="install-dismiss" class="install-btn install-btn-secondary">Позже</button>
      </div>
    </div>
  `;

  let deferredPrompt = null;

  const showBanner = (isIOSBanner = false) => {
    if (sessionStorage.getItem(STORAGE_KEY)) {
      return;
    }
    banner.dataset.ios = isIOSBanner ? "true" : "false";
    if (isIOSBanner) {
      banner.querySelector(".install-banner-text").textContent =
        "Добавить на экран: Поделиться → «На экран Домой»";
      banner.querySelector("#install-confirm").textContent = "Понятно";
      banner.querySelector("#install-confirm").dataset.action = "dismiss";
    }
    container?.appendChild(banner);
  };

  const hideBanner = () => {
    banner.remove();
    sessionStorage.setItem(STORAGE_KEY, "1");
  };

  banner.querySelector("#install-dismiss")?.addEventListener("click", hideBanner);

  banner.querySelector("#install-confirm")?.addEventListener("click", async () => {
    if (banner.dataset.ios === "true" || banner.querySelector("#install-confirm")?.dataset.action === "dismiss") {
      hideBanner();
      return;
    }
    if (deferredPrompt) {
      deferredPrompt.prompt();
      const { outcome } = await deferredPrompt.userChoice;
      if (outcome === "accepted") {
        onStatus?.("Приложение установлено");
      }
      deferredPrompt = null;
      hideBanner();
    }
  });

  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    deferredPrompt = e;
    showBanner(false);
  });

  if (isIOS()) {
    const timer = window.setTimeout(() => showBanner(true), 2000);
    return () => window.clearTimeout(timer);
  }

  const isAndroid = /Android/.test(navigator.userAgent);
  if (isAndroid) {
    const timer = window.setTimeout(() => {
      if (!deferredPrompt && !sessionStorage.getItem(STORAGE_KEY)) {
        banner.querySelector(".install-banner-text").textContent =
          "Добавить на главный экран: меню ⋮ → «Установить приложение»";
        banner.querySelector("#install-confirm").textContent = "Понятно";
        banner.querySelector("#install-confirm").dataset.action = "dismiss";
        showBanner(false);
      }
    }, 3000);
    return () => window.clearTimeout(timer);
  }

  return () => {};
}
