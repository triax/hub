import { useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip from "../../models/Equip";
import EquipRepo, { CustodyRepo } from "../../repository/EquipRepo";

export default function Report(props) {
  const { startLoading, stopLoading } = props;
  const [equips, setEquips] = useState<Equip[]>([]);
  const [ids, setIDs] = useState<number[]>([]);
  const toggle = (y) => y ? (id) => setIDs(ids.filter(i => i != id)) : (id) => setIDs(ids.concat([id]));
  const repo = useMemo(() => new EquipRepo(), []);
  const router = useRouter();
  equips.length ? stopLoading() : startLoading();
  useEffect(() => {
    repo.list().then(setEquips);
  }, [repo]);
  return (
    <Layout {...props}>
      <h1 className="my-4 text-2xl font-bold">何を持って帰ってくれましたか?</h1>
      <div className="w-full">
        {equips.sort(Equip.sort).map(e => <EquipCard
          key={e.id} equip={e}
          selected={ids.includes(e.id)}
          toggle={toggle(ids.includes(e.id))}
        />)}
      </div>

      <div className="w-full text-center">
        <button
          onClick={() => {
            const li = equips.filter(e => ids.includes(e.id)).map(e => `・${e.name}`);
            if (!window.confirm(`以下のアイテムでよかったですか?\n${li.join("\n")}`)) return;
            (new CustodyRepo()).report(ids, props.myself, "").then(() => {
              window.alert("Thank you!!");
              router.push("/");
            });
          }}
          disabled={ids.length == 0}
          className={`w-5/6 text-xl text-white p-8 rounded-md ` + (ids.length ? `bg-blue-600` : `bg-gray-200`)}
        >
          {ids.length ? `上記${ids.length}個の備品回収を記録する` : `選択してください`}
        </button>
      </div>
    </Layout>
  )
}

function EquipCard({ equip, selected, toggle }: { equip: Equip, selected: boolean, toggle: (number) => void }) {
  const card = selected ? "bg-red-800 text-white shadow-md" : "bg-gray-100"
  return (
    <div
      onClick={() => toggle(equip.id)}
      className={`flex rounded-lg px-4 py-2 mb-6 ${card}`}
    >
      <input
        onClick={() => toggle(equip.id)}
        className="mr-4 leading-tight w-6 h-6" type="checkbox"
        checked={selected}
        readOnly={true}
      />
      <span
        className="text-xl"
      >{equip.name}</span>
    </div>
  )
}