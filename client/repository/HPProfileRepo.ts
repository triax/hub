import HPProfile from "../models/HPProfile";
import { fetchJSON } from "./fetch";

const ALLOWED_MIME_TYPES = ["image/png", "image/jpeg", "image/gif", "image/webp"];
const MAX_BYTES = 10 * 1024 * 1024; // 10MB

export function validatePhotoFile(file: File): string | null {
  if (!ALLOWED_MIME_TYPES.includes(file.type)) {
    return `非対応の画像形式です (${file.type})。PNG / JPEG / GIF / WebP のみ使用できます。`;
  }
  if (file.size > MAX_BYTES) {
    return `ファイルサイズが上限 (10MB) を超えています。`;
  }
  return null;
}

export default class HPProfileRepo {
  constructor(public baseURL = import.meta.env.VITE_API_BASE_URL || "") {}

  get(memberId: string): Promise<HPProfile> {
    return fetchJSON<HPProfile>(this.baseURL + `/api/1/members/${memberId}/hp-profile`);
  }

  update(memberId: string, profile: Partial<HPProfile>): Promise<HPProfile> {
    return fetchJSON<HPProfile>(this.baseURL + `/api/1/members/${memberId}/hp-profile`, {
      method: "PUT",
      body: JSON.stringify(profile),
    });
  }

  async uploadPhoto(
    memberId: string,
    type: "formal" | "casual" | "additional",
    file: File
  ): Promise<{ url: string }> {
    const error = validatePhotoFile(file);
    if (error) throw new Error(error);

    const form = new FormData();
    form.append("photo", file);
    const res = await fetch(
      this.baseURL + `/api/1/members/${memberId}/hp-profile/photo?type=${type}`,
      { method: "POST", body: form }
    );
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${res.status}`);
    }
    return res.json();
  }
}
