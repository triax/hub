
interface SlackMember {
  id: string,
  profile: {
    name: string,
    real_name: string,
    display_name: string,
    image_512: string,
    title: string,
  },
  name?: string,
  real_name?: string,
  is_admin: boolean,
  deleted: boolean,
}

interface SlackTeam {
  id: string,
  name: string,
  domain: string,
  // email_domain: string,
  icon: {
    image_132: string,
    image_68: string,
  }
}

export default class Member {
  constructor(
      public slack: SlackMember,
      public number: number = null,
      public status: string = "active",
      public team: SlackTeam = null,
  ) { }

  static fromAPIResponse({slack, number, status, team}): Member {
    return new Member(slack, number, status, team);
  }
  static listFromAPIResponse(res: { slack, number, status, team }[]): Member[] {
    return res.map(Member.fromAPIResponse);
  }
  static placeholder(): Member {
    return new Member({
      id: "xxx", profile: { name: "", real_name: "", display_name: "", image_512: "", title: "" }, is_admin: false, deleted: false
    });
  }
}

export enum MemberStatus {
  Active = "active",
  Limited = "limited",
  Inactive = "inactive",
  // Deleted = "deleted", // 使わない
}