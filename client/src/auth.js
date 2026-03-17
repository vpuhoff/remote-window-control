const STORAGE_KEY = "share-app-access-token";

export async function bootstrapAuth() {
  const url = new URL(window.location.href);
  const secret = url.searchParams.get("secret");
  let token = window.localStorage.getItem(STORAGE_KEY);

  if (secret) {
    const response = await fetch(`/api/session?secret=${encodeURIComponent(secret)}`, {
      method: "POST",
    });

    if (!response.ok) {
      throw new Error("Не удалось обменять secret на access token.");
    }

    const payload = await response.json();
    token = payload.access_token;
    window.localStorage.setItem(STORAGE_KEY, token);
    url.searchParams.delete("secret");
    window.history.replaceState({}, document.title, url.pathname + url.search + url.hash);
  }

  return token;
}

export function clearToken() {
  window.localStorage.removeItem(STORAGE_KEY);
}
