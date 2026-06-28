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
  // 判定順序はサーバ events.go の Tags() と一致させること。
  tags(): EventTag[] {
    const t = this.google.title;
    const result: EventTag[] = [];
    if (/[＃#]練習/.test(t)) result.push("練習");
    if (/[＃#]試合/.test(t)) result.push("試合");
    if (/[＃#]ignore/.test(t)) result.push("ignore");
    if (/[＃#](meeting|mtg)/.test(t)) result.push("meeting");
    if (/[＃#]event/.test(t)) result.push("event");
    if (/[＃#](sponsor|スポンサー)/.test(t)) result.push("sponsor");
    if (result.length === 0) result.push("UNKNOWN");
    return result;
  }
}
