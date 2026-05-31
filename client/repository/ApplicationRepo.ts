const BASE = import.meta.env.VITE_API_BASE_URL || "";

export default class ApplicationRepo {
  async submit(data: {
    type: string;
    email: string;
    name: string;
    fields: Record<string, string>;
    consent_agreed_at: string;
  }) {
    const res = await fetch(`${BASE}/api/1/applications`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async list(type?: string) {
    const url = type
      ? `${BASE}/api/1/applications?type=${encodeURIComponent(type)}`
      : `${BASE}/api/1/applications`;
    const res = await fetch(url);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async update(id: string, data: { steps?: { key: string; label: string; done: boolean }[]; done?: boolean }) {
    const res = await fetch(`${BASE}/api/1/applications/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }
}
