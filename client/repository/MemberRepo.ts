import Member from "../models/Member";
import { fetchJSON } from "./fetch";

export default class MemberRepo {
  constructor(
      public baseURL = import.meta.env.VITE_API_BASE_URL || "",
  ) {}
  myself(): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/myself` + `?t=${Date.now()}`;
    return fetchJSON(endpoint).then(Member.fromAPIResponse);
  }
  get(id: string): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/members/${id}`;
    return fetchJSON(endpoint).then(Member.fromAPIResponse);
  }
  list(params: { cached?: boolean, keyword?: string } = { cached: false }): Promise<Member[]> {
    const query = new URLSearchParams();
    if (params.cached) query.set("cached", "1");
    if (params.keyword) query.set("keyword", params.keyword);
    const endpoint = this.baseURL + `/api/1/members` + '?' + query.toString();
    return fetchJSON(endpoint).then(Member.listFromAPIResponse);
  }
  update(id: string, props: { status?: string, number?: number }): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/members/${id}/props`;
    return fetchJSON(endpoint, {
      method: "POST",
      body: JSON.stringify(props),
    }).then(Member.fromAPIResponse);
  }
}

export class MemberCache extends MemberRepo {
  private static dict: Record<string, Member> = {};
  get(id: string): Promise<Member> {
    if (MemberCache.dict[id]) {
      return Promise.resolve(MemberCache.dict[id]);
    }
    return super.get(id).then(member => {
      MemberCache.dict[id] = member;
      return Promise.resolve(member);
    });
  }
  list(params: { cached: boolean } = { cached: false }): Promise<Member[]> {
    if (params.cached && Object.keys(MemberCache.dict).length) {
      return Promise.resolve(Object.values(MemberCache.dict));
    }
    return super.list(params).then(members => {
      if (params.cached) members.map(m => MemberCache.dict[m.slack.id] = m);
      return Promise.resolve(members);
    });
  }

  /**
   * Get member sync.
   * @param {string} id
   * @returns {Member}
   */
  static pick(id: string): Member {
    return this.dict[id];
  }
}
