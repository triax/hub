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
  // {{{ FIXME: 要らない
  name: string;
  title: string;
  picture: string;
  // }}}
  member?: Member;
}

enum ParticipationType {
  JOIN = "join",
  JOIN_LATE = "join_late",
  LEAVE_EARLY = "leave_early",
  ABSENT = "absent",
}

export type EventTag = "練習" | "試合" | "event" | "meeting" | "UNKNOWN";

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
  tag(): EventTag {
    const t = this.google.title;
    if (/[＃#]練習/.test(t)) return "練習";
    if (/[＃#]試合/.test(t)) return "試合";
    if (/[＃#]event/.test(t)) return "event";
    if (/[＃#]meeting|mtg/.test(t)) return "meeting";
    return "UNKNOWN";
  }
}
