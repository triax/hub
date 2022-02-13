import { useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Equip from "../../models/Equip";
import EquipRepo from "../../repository/EquipRepo";

export default function Report(props) {
  const [equips, setEquips] = useState<Equip[]>([]);
  const repo = useMemo(() => new EquipRepo(), []);
  const router = useRouter();
  useEffect(() => {
    repo.list().then(setEquips);
  }, [repo]);

  return (
    <Layout {...props}>
      <h1 className="my-4 text-2xl font-bold">何を持って帰ってくれましたか？</h1>
    </Layout>
  )
}