export async function fetchWindows(token) {
  const res = await fetch("/api/windows", {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) {
    throw new Error("Не удалось загрузить список окон");
  }
  return res.json();
}

export async function setTargetWindow(token, handle) {
  const res = await fetch("/api/target-window", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ handle }),
  });
  if (!res.ok) {
    throw new Error("Не удалось выбрать окно");
  }
  return res.json();
}
