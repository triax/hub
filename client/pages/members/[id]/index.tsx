import Layout from "../../../components/layout";
import { useRouter } from 'next/router';
import { useEffect, useRef, useState } from "react";
import StatusBadges from "../../../components/statusbadges";

export default function Member(props) {
  const id = useRouter().query.id;
  const [member, setMember] = useState(null);
  useEffect(() => {
    if (!id) return;
    // TODO: Repositoryつくる
    const endpoint = process.env.API_BASE_URL + `/api/1/members/${id}`;
    fetch(endpoint).then(res => res.json()).then(res => setMember(res));
  }, [id]);
  const updateMember = (params) => {
    // TODO:
    // const endpoint = process.env.API_BASE_URL + `/api/1/members/${id}`;
    // fetch(endpoint, { method: "POST", body: JSON.stringify(params) }).then(res => res.json()).then(res => setMember(res));
  }
  const refs = {
    number: useRef<HTMLInputElement>(),
  };
  if (!member) return <></>;
  return (
    <Layout {...props}>
      <div className="flex space-x-4">
        <div className="fle w-44">
          <img
            className="rounded-md"
            src={member.slack.profile.image_512} alt={member.slack.name}
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
            defaultValue={member.number}
            className="flex-grow form-input border-transparent bg-gray-100 rounded-md"
            placeholder="0~99を選択"
            min="0" max="99" step="1"
          />
          <button
            role="button" className="border rounded-md px-4 py-2 cursor-pointer"
            onClick={() => updateMember({ number: refs.number.current.value })}
          >設定</button>
        </div>
      </div>
      <div className="py-2">
        <div className="flex space-x-2"><StatusBadges member={member} size="text-lg px-4 py-1" /></div>
      </div>
      <div className="p-12 flex justify-center items-center">
        <a href="/members" className="underline">一覧に戻る</a>
      </div>
    </Layout>
  )
}