package httpserver

const hostUIHTML = `<!doctype html>
<html lang="ru">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Share Host Control</title>
    <style>
      body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; margin: 0; padding: 24px; }
      h1 { margin-top: 0; }
      .row { display: flex; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
      button { background: #2563eb; color: white; border: 0; border-radius: 8px; padding: 10px 14px; cursor: pointer; }
      .secondary { background: #334155; }
      .card { background: #111827; border: 1px solid #334155; border-radius: 12px; padding: 12px 14px; margin-bottom: 10px; }
      .muted { color: #94a3b8; font-size: 0.9rem; }
      #status { margin-bottom: 16px; color: #67e8f9; }
      code { color: #cbd5e1; }
    </style>
  </head>
  <body>
    <h1>Выбор целевого окна</h1>
    <div id="status">Загрузка...</div>
    <div class="row">
      <button id="refresh">Обновить список</button>
    </div>
    <div id="current" class="card"></div>
    <div id="windows"></div>
    <script>
      const status = document.getElementById("status");
      const current = document.getElementById("current");
      const windowsRoot = document.getElementById("windows");

      async function loadCurrent() {
        const response = await fetch("/api/target-window");
        const payload = await response.json();
        if (!payload.selected) {
          current.innerHTML = "<strong>Текущее окно:</strong> не выбрано";
          return;
        }
        current.innerHTML =
          "<strong>Текущее окно:</strong> " + escapeHtml(payload.selected.title) +
          "<div class='muted'>" + escapeHtml(payload.selected.process_name) + " | HWND " + payload.selected.handle + "</div>";
      }

      async function loadWindows() {
        status.textContent = "Обновление списка окон...";
        const response = await fetch("/api/windows");
        const windows = await response.json();
        windowsRoot.innerHTML = "";
        for (const windowItem of windows) {
          const card = document.createElement("div");
          card.className = "card";
          card.innerHTML =
            "<strong>" + escapeHtml(windowItem.title) + "</strong>" +
            "<div class='muted'>" + escapeHtml(windowItem.process_name) + " | HWND " + windowItem.handle + "</div>";
          const button = document.createElement("button");
          button.textContent = "Сделать целевым";
          button.addEventListener("click", async () => {
            status.textContent = "Выбираю окно...";
            const selection = await fetch("/api/target-window", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ handle: windowItem.handle })
            });
            if (!selection.ok) {
              status.textContent = "Не удалось выбрать окно";
              return;
            }
            await loadCurrent();
            status.textContent = "Окно выбрано";
          });
          card.appendChild(document.createElement("div")).appendChild(button);
          windowsRoot.appendChild(card);
        }
        status.textContent = "Список окон обновлен";
      }

      function escapeHtml(value) {
        return String(value)
          .replaceAll("&", "&amp;")
          .replaceAll("<", "&lt;")
          .replaceAll(">", "&gt;")
          .replaceAll('"', "&quot;");
      }

      document.getElementById("refresh").addEventListener("click", loadWindows);
      Promise.all([loadCurrent(), loadWindows()]).catch((error) => {
        status.textContent = error instanceof Error ? error.message : "Ошибка загрузки";
      });
    </script>
  </body>
</html>
`
