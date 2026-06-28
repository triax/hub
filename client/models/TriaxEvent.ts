import Member from "./Member";

interface GoogleEvent {
  id: string;
  title: string;
  location: string;
  start_time: number;
  end_time: number;
}

export interface Participation {
  params?: any;
  type: ParticipationType;
  member?: Member;
}

enum ParticipationType {
  JOIN = "join",
  JOIN_LATE = "join_late",
  LEAVE_EARLY = "leave_early",
  ABSENT = "absent",
}

export type EventTag = "練習" | "試合" | "event" | "meeting" | "sponsor" | "ignore" | "UNKNOWN";

// TAG_PATTERNS はタグ判定の唯一の定義（判定順序込み）。
// サーバ events.go の tagDefs と順序・正規表現を一致させること。
const TAG_PATTERNS: { tag: EventTag; re: RegExp }[] = [
  { tag: "練習", re: /[＃#]練習/ },
  { tag: "試合", re: /[＃#]試合/ },
  { tag: "ignore", re: /[＃#]ignore/ },
  { tag: "meeting", re: /[＃#](meeting|mtg)/ },
  { tag: "event", re: /[＃#]event/ },
  { tag: "sponsor", re: /[＃#](sponsor|スポンサー)/ },
];

export default class TeamEvent {
  constructor(
      public google: GoogleEvent,
      public participations: Record<string, Participation>,
  ) { }
  static fromAPIResponse({google, participations_json_str}): TeamEvent {
    const pats: Record<string, Participation> = JSON.parse(participations_json_str || "{}");
    return new TeamEvent(google, pats);
  }
  static placeholder(): TeamEvent {
    return new TeamEvent({ id: '', title: 'xx', location: 'xxx', start_time: 0, end_time: 0 }, {});
  }
  // tags はタイトルに含まれる全てのタグを返す（複数タグ対応）。
  // 該当タグが無ければ "UNKNOWN" ひとつを返す。
  tags(): EventTag[] {
    const result = TAG_PATTERNS.filter(p => p.re.test(this.google.title)).map(p => p.tag);
    return result.length ? result : ["UNKNOWN"];
  }
  // hasTag は指定タグがタイトルに含まれるかを返す（tags() と一貫）。
  hasTag(tag: EventTag): boolean {
    return this.tags().includes(tag);
  }
}
