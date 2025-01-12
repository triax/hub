import { Dialog } from "@headlessui/react";
import Member from "../../models/Member";
import { PlayerNumber } from "../../models/PlayerNumber";
import { useState } from "react";
import { PlayerNumberRepo } from "../../repository/PlayerNumberRepo";

function QueryFieldView({ query, setQuery }: { query: string, setQuery: (q: string) => void }) {
  return (
    <input
      type="text"
      value={query}
      onChange={(e) => setQuery(e.target.value)}
      placeholder="名前やポジションでフィルタ"
      className="w-full p-2 border border-gray-300 rounded-md mb-2"
    />
  );
}

function filterfunc(m: Member, query: string): boolean {
  if (m.slack.deleted) return false;
  if (query === "") return true;
  const q = query.toLowerCase();
  if (m.slack.id.toLowerCase().includes(q)) return true;
  if (m.slack.name.toLowerCase().includes(q)) return true;
  if (m.slack.real_name.toLowerCase().includes(q)) return true;
  if (m.slack.profile.display_name.toLowerCase().includes(q)) return true;
  if (m.slack.profile.real_name.toLowerCase().includes(q)) return true;
  if (m.slack.profile.title.toLowerCase().includes(q)) return true;
  return false;
}

function MemberListView({
  members, query, commit,
  previousassign
}: {
  members: { [slack_id: string]: Member }, query: string, commit: (m: Member, deprive?: boolean) => void,
  previousassign?: Member;
}) {
  return (
    <div className="space-y-2 h-96 overflow-y-auto">
      {Object.values(members).filter(m => filterfunc(m, query)).map((m) => (
        <div key={m.slack.id} className="flex space-x-2 items-center">
          <div>
            <img src={m.slack.profile.image_512} className="w-8 rounded-sm"
              alt={m.slack.real_name}
            />
          </div>
          <div className="flex-1">{m.slack.real_name}</div>
          <div className="w-12 overflow-y-scroll whitespace-nowrap">{m.slack.profile.title}</div>
          <div className="">
            {previousassign?.slack?.id == m.slack.id ?
              <button className="bg-red-400 text-white flex justify-center items-center py-1 px-2"
                onClick={() => commit(m, true)}
              >割当を外す</button>
              :
              <button className="bg-blue-200 text-white flex justify-center items-center py-1 px-2"
                onClick={() => commit(m)}
              >割り当てる</button>
            }
          </div>
        </div>
      ))}
    </div>
  );
}

export function NumAssignModalContent({
  close, members, playernumber,
  previousassign,
  loading,
}: {
  close: () => void;
  members: { [slack_id: string]: Member };
  playernumber: PlayerNumber;
  previousassign?: Member;
  loading: { start: () => void, stop: () => void };
}) {
  const [query, setQuery] = useState("");
  return (
    <div
      className="inline-block w-full max-w-md p-6 my-8 overflow-hidden text-left align-middle transition-all transform bg-white shadow-xl rounded-2xl"
    >
      <Dialog.Title as="h3" className="font-medium leading-6 text-gray-900 mb-2">
        <span>背番号</span> <span className="text-4xl text-red-900 border p-1">{playernumber.number}</span> <span>を選手に割り当てる</span>
      </Dialog.Title>

      <div>
        <QueryFieldView query={query} setQuery={setQuery} />
        <MemberListView members={members} query={query} previousassign={previousassign}
          commit={async (member, deprive = false) => {
            let message = `背番号${playernumber.number}を\n${member.slack.real_name}に割り当てますか？`;
            if (previousassign) message += `\n\nこれにより、${previousassign.slack.real_name}から背番号${playernumber.number}の割り当てを外します。`;
            if (deprive) message = `背番号${playernumber.number}の割り当てを${previousassign.slack.real_name}から外しますか？`;
            if (!window.confirm(message)) return;
            close();
            loading.start();
            const repo = new PlayerNumberRepo();
            await repo.assign(playernumber, member.slack.id, deprive);
            loading.stop();
            // refresh the page data
            location.reload();
          }}
        />
      </div>
      <div>
        <button onClick={close} className="mt-4 bg-red-200 text-gray-800 py-1 px-4 rounded-md">やっぱりやめる</button>
      </div>
    </div>
  );
}