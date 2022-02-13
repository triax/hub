import { useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";
import { TrashIcon } from "@heroicons/react/outline";

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
  console.log(equip);
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