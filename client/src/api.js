import { refreshToken } from "./auth.js";

async function fetchWithRetry(url, options, token) {
  let res = await fetch(url, { ...options, headers: { ...options?.headers, Authorization: `Bearer ${token}` } });
  if (res.status === 401) {
    const newToken = await refreshToken();
    if (newToken) res = await fetch(url, { ...options, headers: { ...options?.headers, Authorization: `Bearer ${newToken}` } });
  }
  return res;
}

export async function fetchWindows(token) {
  const res = await fetchWithRetry("/api/windows", {}, token);
  if (!res.ok) throw new Error("Не удалось загрузить список окон");
  return res.json();
}

export async function setTargetWindow(token, handle) {
  const res = await fetchWithRetry("/api/target-window", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ handle }),
  }, token);
  if (!res.ok) throw new Error("Не удалось выбрать окно");
  return res.json();
}
