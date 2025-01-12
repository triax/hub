import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import { PlayerNumberRepo } from "../../repository/PlayerNumberRepo";
import { useRouter } from "next/router";
import { PlayerNumber } from "../../models/PlayerNumber";
import Member from "../../models/Member";
import { Dialog } from "@headlessui/react";
import { NumAssignModalContent } from "../../components/Uniforms/ModalContents";

async function listMembers(incdel: boolean): Promise<Member[]> {
  const endpoint = process.env.API_BASE_URL + "/api/1/members";
  const res = await fetch(endpoint + (incdel ? "?include_deleted=1" : ""));
  return (await res.json()).map((m) => Member.fromAPIResponse(m));
}

function PlayerNumberListView({ playernumbers, members, loading }: {
  playernumbers: PlayerNumber[];
  members: { [slack_id: string]: Member };
  loading: { start: () => void, stop: () => void };
}) {
  const [modalContent, setModalContante] =  useState<JSX.Element | null>(null);
  return (
    <div className="divide-y">
      <div className="flex space-x-2 justify-center items-center">
        <div className="w-8">#</div>
        <div className="flex-1">ユニフォーム</div>
        <div>割り当て</div>
      </div>
      {playernumbers.sort((p, n) => p.number > n.number ? 1 : -1).map((p) => (
        <div key={p.number} className="flex space-x-2 justify-center items-center py-1">
          <div className="w-8">{p.number}</div>
          <div className="flex-1">
            {(p.uniforms || []).map((u, i) => <div key={i}></div>)}
            <button className="bg-gray-200 hover:bg-blue-200 text-white w-8 h-8 flex justify-center items-center">+</button>
          </div>
          <button className="bg-gray-200 hover:bg-blue-200 text-white rounded-full w-8 h-8 flex justify-center items-center"
            onClick={() => setModalContante(<NumAssignModalContent
              playernumber={p}
              close={() => setModalContante(null)}
              members={members}
              loading={loading}
              previousassign={members[p.player_id]}
            />)}
          >
            {p.player_id && members[p.player_id] ? <img src={members[p.player_id].slack.profile.image_512} 
              alt={members[p.player_id].slack.real_name}
            /> : "+"}
          </button>
        </div>
      ))}
      <Dialog
        open={modalContent !== null}
        as="div"
        className="fixed inset-0 z-10 overflow-y-auto"
        onClose={() => setModalContante(null)}
      >
        <div className="min-h-screen px-4 text-center">
          <Dialog.Overlay className="fixed inset-0 bg-black bg-opacity-40" />
          {/* This element is to trick the browser into centering the modal contents. */}
          <span className="inline-block h-screen align-middle" aria-hidden="true">&#8203;</span>
          {modalContent}
        </div>
      </Dialog>
    </div>
  );
}

function UniformClothesListView({ uniforms, members }: {
  uniforms: any[];
  members: { [slack_id: string]: Member };
}) {
  return (
    <div>
      {uniforms.map((u) => (
        <div key={u.id} className="flex space-x-2">
          <div>{u.number}</div>
          <div>{u.size}</div>
          <div>{u.color}</div>
          <div>{u.damaged}</div>
          <div>{u.owner_id}</div>
        </div>
      ))}
    </div>
  );
}

export default function Uniforms(props) {
  const repo = useMemo(() => new PlayerNumberRepo(), []);
  const [playernumbers, setPlayernumbers] = useState<PlayerNumber[]>([]);
  const [members, setMembers] = useState<{[slack_id:string]:Member}>({})
  useEffect(() => {
    listMembers(true).then(mems => setMembers(mems.reduce((acc, mem) => ({...acc, [mem.slack.id]: mem}), {})));
    repo.all().then(a => setPlayernumbers(PlayerNumber.fromResponse(a)));
  }, [repo]);
  const active = `shadow-inner rounded-t-lg bg-blue-600 text-white`;
  const inactive = `shadow rounded-t-lg text-gray-600`;
  const router = useRouter();
  const hash = router.asPath.split("#")[1];
  return (
    <Layout {...props}>
      <div className="flex space-x-2">
        <div className={"flex-1 p-4 text-center cursor-pointer " + (hash !== "clothes" ? active : inactive)}
          onClick={() => router.push("/uniforms#numbers")}
        >背番号</div>
        <div className={"flex-1 p-4 text-center cursor-pointer " + (hash === "clothes" ? active : inactive)}
          // onClick={() => props.router.push("/uniforms")}
          onClick={() => router.push("/uniforms#clothes")}
        >ユニフォーム</div>
      </div>
      <div>
        {hash === "clothes" ? <UniformClothesListView
          uniforms={[]}
          members={members}
        /> : <PlayerNumberListView
          playernumbers={playernumbers}
          members={members}
          loading={{ start: props.startLoading, stop: props.stopLoading }}
        />}
      </div>
    </Layout>
  );
}