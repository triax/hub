export interface CustomField {
  key: string;
  value: string;
  hidden: boolean;
}

export default interface HPProfile {
  display_name: string;
  display_name_kana: string;
  first_name: string;
  family_name: string;
  height: number;
  weight: number;
  position: string;
  hometown: string;
  school: string;
  bio: string;
  custom_fields: CustomField[];
  portrait_formal_url: string;
  portrait_casual_url: string;
  additional_photo_urls: string[];
  hide_from_hp: boolean;
  hidden_fields: string[];
}

export const HIDDEN_FIELD_KEYS = [
  "display_name",
  "display_name_kana",
  "first_name",
  "family_name",
  "height",
  "weight",
  "position",
  "hometown",
  "school",
  "bio",
  "portrait_formal",
  "portrait_casual",
] as const;

export type HiddenFieldKey = (typeof HIDDEN_FIELD_KEYS)[number];

export function emptyHPProfile(): HPProfile {
  return {
    display_name: "",
    display_name_kana: "",
    first_name: "",
    family_name: "",
    height: 0,
    weight: 0,
    position: "",
    hometown: "",
    school: "",
    bio: "",
    custom_fields: [],
    portrait_formal_url: "",
    portrait_casual_url: "",
    additional_photo_urls: [],
    hide_from_hp: false,
    hidden_fields: [],
  };
}
