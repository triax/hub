export default interface HPProfile {
  height: number;
  weight: number;
  position: string;
  hometown: string;
  school: string;
  faculty: string;
  bio: string;
  portrait_formal_url: string;
  portrait_casual_url: string;
  additional_photo_urls: string[];
  hide_from_hp: boolean;
  hidden_fields: string[];
}

export const HIDDEN_FIELD_KEYS = [
  "height",
  "weight",
  "position",
  "hometown",
  "school",
  "faculty",
  "bio",
  "portrait_formal",
  "portrait_casual",
  "additional_photos",
] as const;

export type HiddenFieldKey = (typeof HIDDEN_FIELD_KEYS)[number];

export function emptyHPProfile(): HPProfile {
  return {
    height: 0,
    weight: 0,
    position: "",
    hometown: "",
    school: "",
    faculty: "",
    bio: "",
    portrait_formal_url: "",
    portrait_casual_url: "",
    additional_photo_urls: [],
    hide_from_hp: false,
    hidden_fields: [],
  };
}
