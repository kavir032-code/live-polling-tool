const API = import.meta.env.VITE_API_URL || "http://localhost:8080";

export async function request(path, options = {}) {
  const token = localStorage.getItem("token");
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };

  if (token) headers.Authorization = `Bearer ${token}`;

  const response = await fetch(`${API}${path}`, {
    ...options,
    headers,
  });

  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || "Request failed");
  }
  return data;
}

export function websocketURL(pollID) {
  const base = API.replace(/^http/, "ws");
  return `${base}/api/polls/${pollID}/ws`;
}
