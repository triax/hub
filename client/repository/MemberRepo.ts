import Member from "../models/Member";

export default class MemberRepo {
  constructor(
      public baseURL = process.env.API_BASE_URL,
  ) {}
  myself(): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/myself` + `?t=${Date.now()}`;
    return fetch(endpoint).then(res => res.json()).then(Member.fromAPIResponse);
  }
  get(id: string): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/members/${id}`;
    return fetch(endpoint).then(res => res.json()).then(Member.fromAPIResponse);
  }
  list(params: { cached: boolean } = { cached: false }): Promise<Member[]> {
    const query = new URLSearchParams();
    if (params.cached) query.set("cached", "1");
    const endpoint = this.baseURL + `/api/1/members` + '?' + query.toString();
    return fetch(endpoint).then(res => res.json()).then(Member.listFromAPIResponse);
  }
  update(id: string, props: { status?: string, number?: number }): Promise<Member> {
    const endpoint = this.baseURL + `/api/1/members/${id}/props`;
    return fetch(endpoint, {
      method: "POST",
      body: JSON.stringify(props),
    }).then(res => res.json()).then(Member.fromAPIResponse);
  }
}

export class MemberCache extends MemberRepo {
  private static dict: Record<string, Member> = {};
  get(id: string): Promise<Member> {
    if (MemberCache.dict[id]) {
      // console.info("Cache hit");
      return Promise.resolve(MemberCache.dict[id]);
    }
    return super.get(id).then(member => {
      MemberCache.dict[id] = member;
      return Promise.resolve(member);
    });
  }
}
