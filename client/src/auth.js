const STORAGE_KEY_TOKEN = "share-app-access-token";
const STORAGE_KEY_SECRET = "share-app-secret";

async function exchangeSecret(secret) {
  const res = await fetch(`/api/session?secret=${encodeURIComponent(secret)}`, { method: "POST" });
  if (!res.ok) throw new Error("Не удалось обменять secret на access token.");
  const payload = await res.json();
  window.localStorage.setItem(STORAGE_KEY_TOKEN, payload.access_token);
  return payload.access_token;
}

export async function bootstrapAuth() {
  const url = new URL(window.location.href);
  const secretFromUrl = url.searchParams.get("secret");
  let token = window.localStorage.getItem(STORAGE_KEY_TOKEN);

  if (secretFromUrl) {
    window.localStorage.setItem(STORAGE_KEY_SECRET, secretFromUrl);
    token = await exchangeSecret(secretFromUrl);
    url.searchParams.delete("secret");
    window.history.replaceState({}, document.title, url.pathname + url.search + url.hash);
  } else if (!token && window.localStorage.getItem(STORAGE_KEY_SECRET)) {
    token = await refreshToken();
  }

  return token;
}

export async function refreshToken() {
  const secret = window.localStorage.getItem(STORAGE_KEY_SECRET);
  if (!secret) return null;
  try {
    return await exchangeSecret(secret);
  } catch {
    return null;
  }
}

export function getStoredToken() {
  return window.localStorage.getItem(STORAGE_KEY_TOKEN);
}

export function clearToken() {
  window.localStorage.removeItem(STORAGE_KEY_TOKEN);
}
