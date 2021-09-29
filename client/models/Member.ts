
interface SlackMember {
  id: string,
  profile: {
    name: string,
    real_name: string,
    image_512: string,
    title: string,
  },
  is_admin: boolean,
  deleted: boolean,
}

export default class Member {
  constructor(
      public slack: SlackMember,
      public number: number = null,
      public status: string = "active",
  ) { }

  static fromAPIResponse({slack, number, status}): Member {
    return new Member(slack, number, status);
  }
  static listFromAPIResponse(res: {slack, number, status}[]): Member[] {
    return res.map(Member.fromAPIResponse);
  }
  static placeholder(): Member {
    return new Member({
      id: "xxx", profile: { name: "", real_name: "", image_512: "", title: "" }, is_admin: false, deleted: false
    });
  }
}

export enum MemberStatus {
  Active = "active",
  Limited = "limited",
  Inactive = "inactive",
  // Deleted = "deleted", // 使わない
}