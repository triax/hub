import { useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import Layout from "../../components/layout";
import StatusBadges from "../../components/statusbadges";
import { useAppContext } from "../context";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "";

async function listMembers(incdel: boolean) {
  const endpoint = API_BASE_URL + "/api/1/members";
  const res = await fetch(endpoint + (incdel ? "?include_deleted=1" : ""));
  return res.json();
}

export default function Members() {
  const { myself } = useAppContext();
  const search: Record<string, string> = useSearch({ strict: false });
  const incdel = search.include_deleted == "1";
  const [members, setMembers] = useState([]);
  useEffect(() => {
    listMembers(incdel).then(mems => setMembers(mems));
  }, [incdel]);
  return (
    <Layout>
      <div className="divide-y divide-gray-100">
        <List>
          {members.map(member => <MemberItem
            key={member.slack.id}
            member={member}
            admin={myself.slack.is_admin}
          />)}
        </List>
      </div>
    </Layout>
  )
}

function List({ children }) {
  return (
    <ul className="divide-y divide-gray-100">
      {children}
    </ul>
  )
}

function MemberItem({ member, admin }) {
  const { slack } = member;
  return (
    <article className="py-2 flex space-x-4 cursor-pointer"
      onClick={() => location.href = `/members/${member.slack.id}`}
    >
      <div className="flex-none w-12 h-12">
        <img
          src={slack.profile.image_512}
          alt={slack.profile.real_name}
          className="flex-none w-12 h-12 rounded-md object-cover bg-gray-100"
        />
      </div>
      <div className="min-w-0 relative flex-auto sm:pr-20 lg:pr-0 xl:pr-20 flex">
        <div className="flex-1">
          <h3 className="text-md text-black">
            {slack.profile.real_name}
          </h3>
          <div className="flex space-x-4 text-gray-400">
            <div className="flex-shrink-0">#{member.number === null ? "未設定" : member.number}</div>
            <div><PositionCols title={member.slack.profile.title} /></div>
          </div>
        </div>
        <div className="flex-shrink">
          <div className="space-x-1"><StatusBadges member={member} admin={admin} /></div>
        </div>
      </div>
    </article>
  )
}

function PositionCols({ title }) {
  const positions: string[] = (title || "").split(/[\/／,、・]/).filter(Boolean);
  if (!positions || positions.length == 0) return <span>POS未設定</span>;
  return <>{positions.reduce((ctx, pos, i) => {
    ctx.push(<span key={pos}>{pos}</span>);
    ctx.push(<span key={i} className="px-1">/</span>);
    return ctx;
  }, []).slice(0, -1)}</>;
}
