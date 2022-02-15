import Image from "next/image";
import { NextRouter, useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip, { Custody } from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";
import { MemberCache } from "../../repository/MemberRepo";

export default function Item(props) {
  const id = useRouter().query.id as string;
  const repo = useMemo(() => new EquipRepo(), []);
  const router = useRouter();
  const [equip, setEquip] = useState<Equip>(null);
  useEffect(() => {
    if (!id) return;
    repo.get(id).then(setEquip);
  }, [repo, id]);
  if (!equip) return <Layout {...props}></Layout>;
  return (
    <Layout {...props}>

      <div className="w-full">
        <div className="bg-white shadow-md rounded px-4 pt-6 pb-8 mb-4">
          <div className="mb-4">
            <h1 className="text-2xl font-bold">{equip.name}</h1>
          </div>
          <div className="mb-4 flex space-x-2">
            {equip.forPractice ? <div className="rounded-md bg-teal-600   text-white px-2">練習で必要</div> : null}
            {equip.forGame     ? <div className="rounded-md bg-orange-600 text-white px-2">試合で必要</div> : null}
          </div>


          <div className="mb-4">
            {equip.description.split("\n").map((line, i) => <div key={i}>{line}</div>)}
          </div>

          <CustodyFeed history={equip.history} router={router} />

        </div>
      </div>

      {props.myself.slack.is_admin ? <div className="w-1/2">
        <div
          onClick={() => {
            if (window.confirm(`「${equip.name}」を削除しますか?\nこのアクションは取り消せません。`)) {
              repo.delete(equip.id).then(() => router.push(`/equips`));
            }
          }}
          className="rounded-md bg-red-600 text-white flex justify-center p-2">
          <span>このアイテムを削除</span>
        </div>
      </div> : null}
    </Layout>
  )
}

function CustodyFeed({history, router}) {
  const cache = useMemo(() => new MemberCache(), []);
  return (
    <div>
      {history.map(custody => <FeedEntry
        key={custody.ts}
        timestamp={custody.ts}
        memberID={custody.member_id}
        comment={custody.comment}
        cache={cache}
        router={router}
      />)}
    </div>
  );
}

function DateRow({timestamp}: {timestamp: number}) {
  const d = new Date(timestamp);
  return (
    <div className="h-10 flex items-center">
      <div>{d.getFullYear()}年</div>
      <div>{d.getMonth()+1}月</div>
      <div className="mr-2">{d.getDate()}日</div>
      <div>{d.getHours()}</div>
      <div>:</div>
      <div>{d.getMinutes()}</div>
    </div>
  )
}

function FeedEntry({ timestamp, memberID, comment, cache, router }: {
  timestamp: number, memberID: string, comment: string, cache: MemberCache, router: NextRouter,
}) {
  const [c, setCustody] = useState<Custody>(null);
  useEffect(() => {
    cache.get(memberID).then(m => {
      setCustody(new Custody(memberID, timestamp, comment, m));
    });
  }, [cache]);
  if (c == null) return null;
  return (
    <div className="flex">
      <div className="flex flex-col items-center">
        <div className="w-10 h-10 rounded-full overflow-hidden">
          <Image
            onClick={() => router.push(`/members/${c.member_id}`)}
            loader={({ src }) => src}
            unoptimized={true}
            src={c.member?.slack?.profile?.image_512}
            alt={c.member?.slack?.profile?.real_name}
            className="flex-none w-12 h-12 rounded-md object-cover bg-gray-100"
            width={120}
            height={120}
          />
        </div>
        <div className="border-x w-0 h-4 my-2 flex-grow border-gray-400" />
      </div>
      <div className="pl-2 pb-4">
        <div className="flex h-10 align-middle text-gray-600">
          <DateRow timestamp={timestamp} />
        </div>
        <div className="">
          {c.comment.split("\n").map((line, i) => <div key={i}>{line}</div>)}
        </div>
      </div>
    </div>
  )
}