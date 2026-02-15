import Layout from "../../components/layout";
import { useParams } from '@tanstack/react-router';
import { ChangeEvent, useEffect, useMemo, useState } from "react";
import StatusBadges from "../../components/statusbadges";
import MemberRepo from "../../repository/MemberRepo";
import Member from "../../models/Member";
import { useAppContext } from "../context";

export default function MemberView() {
  const { myself } = useAppContext();
  const repo = useMemo(() => new MemberRepo(), []);
  const { id } = useParams({ strict: false });
  const [member, setMember] = useState<Member>(null);
  const [num, setNumberInput] = useState<number>(null);
  useEffect(() => { if (id) repo.get(id).then(setMember); }, [id, repo]);
  if (!member) return <></>;

  return (
    <Layout>
      <div className="flex space-x-4">
        <div className="fle w-44">
          <img
            className="rounded-md"
            src={member.slack.profile.image_512} alt={member.slack.profile.name}
          />
        </div>
        <div className="flex-grow">
          <div className="flex flex-col h-full">
            <h1 className="text-3xl font-medium">{member.slack.profile.real_name}</h1>
            <div className="text-2xl flex-grow text-gray-800">{member.slack.profile.title || "ポジション未設定"}</div>
            <div className="flex flex-row-reverse text-gray-400">Slackで編集可</div>
          </div>
        </div>
      </div>
      <div className="py-2">
        <div className="form-group flex items-center space-x-4">
          <span>背番号:</span>
          <input type="number"
            defaultValue={member.number || num}
            onChange={ev => setNumberInput(parseInt(ev.target.value))}
            className="flex-grow form-input border-transparent bg-gray-100 rounded-md"
            placeholder="0~99を選択"
            min="0" max="99" step="1"
          />
          <button
            role="button" className="border rounded-md px-4 py-2 cursor-pointer"
            onClick={() => repo.update(member.slack.id, { number: num })}
          >設定</button>
        </div>
      </div>
      <div className="py-2">
        <div className="flex space-x-2"><StatusBadges member={member} size="text-lg px-4 py-1" /></div>
      </div>

      {myself.slack.is_admin ? <AdminMenu member={member} repo={repo} /> : null}

      <div className="p-12 flex justify-center items-center">
        <a href="/members" className="underline">一覧に戻る</a>
      </div>
    </Layout>
  )
}

function AdminMenu({ member, repo }: { member: Member, repo: MemberRepo }) {
  const { status, slack } = member;
  const onInputChange = async (ev: ChangeEvent<HTMLSelectElement>) => {
    await repo.update(slack.id, { status: ev.target.value });
  };
  return <div className="p-2 border rounded-md bg-red-100">
    <h3>管理者メニュー</h3>
    <div>
      <select
        className="w-full rounded-sm" defaultValue={slack.deleted ? "deleted" : (status || "active")}
        disabled={slack.deleted}
        onChange={onInputChange}
      >
        <option value="active">通常部員</option>
        <option value="limited">練習外部員（出欠回答不要）</option>
        <option value="inactive">休眠</option>
        <option
          value="deleted"
          disabled={!slack.deleted}
        >退部済み（Slackで設定）</option>
      </select>
    </div>
  </div>;
}
