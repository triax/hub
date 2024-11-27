import { useRouter } from "next/router";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../../components/layout";
import { LocationMarkerIcon } from "@heroicons/react/outline";
import { Disclosure } from "@headlessui/react";
import { EventDateTime } from "../../../components/Events";
import TeamEventRepo from "../../../repository/EventRepo";
import { MemberCache } from "../../../repository/MemberRepo";
import Member from "../../../models/Member";
import TeamEvent, { Participation } from "../../../models/TriaxEvent";
import EventRSVPButtonsRow from "../../../components/Events/RSVPButtons";
import { RSVPModal } from "../../../components/Events/RSVPModal";

export default function EventView(props) {
  const {startLoading, stopLoading} = props;
  const evrepo = useMemo(() => new TeamEventRepo(), []);
  const merepo = useMemo(() => new MemberCache(), []);
  const id = useRouter().query.id as string;
  const [event, setEvent] = useState<TeamEvent>(TeamEvent.placeholder());
  const [allMembers, setAllMembers] = useState<Member[]>([]);
  const [modalevent, setModalEvent] = useState(null);

  useEffect(() => {
    if (!id) return;
    evrepo.get(id).then(setEvent);
    merepo.list({cached: true}).then(setAllMembers)
  }, [id, evrepo, merepo]);

  const submit = async function(params) {
    startLoading();
    const updated = await evrepo.rsvp(params);
    setEvent(updated);
    stopLoading();
  }

  if (!event || !event.google || !event.google.id) return <></>;
  if (allMembers.length == 0) return <></>;

  // 集計
  // List of positions of American Football
  const positions = ["OL", "QB", "RB", "WR", "TE", "DL", "LB", "DB", "TRAINER", "STAFF", "OTHERS"];
  const sum: {
    _yes: Record<string, Participation[]>,
    _no: Record<string, Participation[]>,
    unanswered: Member[],
  } = Object.entries(event.participations).reduce((ctx, [id, entry]: [string, Participation]) => {
    const member = MemberCache.pick(id);
    let posi = entry.title.split("/")[0].toUpperCase() || member?.slack?.profile?.title?.split("/")[0].toUpperCase();
    if (!positions.includes(posi)) posi = "OTHERS";
    if (['join', 'join_late', 'leave_early'].includes(entry.type)) {
      ctx._yes[posi] = (ctx._yes[posi] || []).concat([{ ...entry, member }]);
    } else {
      ctx._no[posi] = (ctx._no[posi] || []).concat([{ ...entry, member }]);
    }
    ctx.unanswered = ctx.unanswered.filter(m => m.slack.id !== id);
    return ctx;
  }, { _yes: {}, yes: [], _no: {}, no: [], unanswered: allMembers });

  const onClickDeleteEvent = () => {
    if (!window.confirm(`イベント「${event.google.title}」を削除しますか？\nこの操作は取り消せません。`)) return;
    evrepo.delete(id).then(res => { window.alert(JSON.stringify(res)); (location.href = "/") })
  };

  return (
    <Layout {...props}>
      <div>
        <div>
          <h1 className="text-xl text-gray-800 mb-4">{event.google.title}</h1>
        </div>
        <div className="flex flex-col">
          <div className="flex space-x-2">
            <div className="text-md font-semibold">日時</div>
            <EventDateTime timestamp={event.google.start_time} className="text-gray-800 text-md" /><EndTime end_time={event.google.end_time} />
          </div>
          <div className="flex space-x-2">
            <div className="text-md font-semibold">場所</div>
            <div
              className="text-gray-800 text-md flex-1"
              style={{ wordBreak: "keep-all" }}
            >{event.google.location}</div>
            <div className="flex justify-center items-center w-10">
              <LocationMarkerIcon className="w-full cursor-pointer text-green-600"
                onClick={() => window.open(`https://www.google.com/maps/search/${encodeURIComponent(event.google.location)}`, '_blank')}
              />
            </div>
          </div>
        </div>

        <div className="py-4">
          {event.google.start_time < Date.now() ? null : <EventRSVPButtonsRow
            event={event}
            answer={/* answer*/ event.participations[props.myself.slack.id] || {}}
            setModalEvent={setModalEvent}
            submit={submit}
          />}
        </div>

        <div className="py-4 space-y-12">

          {/* {{{ DEV */}
          <div>
            <div className="border-b">
              <span className="font-semibold">参加</span>
              <span className="px-4">{Object.values(sum._yes).flat().length}人</span>
            </div>
            <div>
              {positions.map(pos => <PositionParticipationSection key={pos} join={true} title={pos} entries={sum._yes[pos] || []} />)}
            </div>
          </div>
          {/* DEV }}} */}

          <div>
            <div className="border-b">
              <span className="font-semibold">不参加</span>
              <span className="px-4">{Object.values(sum._no).flat().length}人</span>
            </div>
            <div className="divide-y">
              {positions.map(pos => <PositionParticipationSection key={pos} join={false} title={pos} entries={sum._no[pos] || []} />)}
            </div>
          </div>

          <div>
            <Disclosure>
              <Disclosure.Button as="div" className="border-b cursor-pointer">
                <span className="font-semibold">未回答</span>
                <span className="px-4">{sum.unanswered.length}人</span>
              </Disclosure.Button>
              <Disclosure.Panel as="div" className="divide-y">
                {sum.unanswered.sort((p, n) => p.slack?.profile.title < n.slack?.profile.title ? 1 : -1).map((m: Member) => (
                  <div key={m.slack.id} className="flex space-x-2 items-center">
                    <div className="flex-auto">{m.slack.real_name}</div>
                    <div className="text-xs">{m.slack?.profile.title}</div>
                  </div>
                ))}
              </Disclosure.Panel>
            </Disclosure>
          </div>
        </div>

        {props.myself.slack.is_admin ? <div className="py-8">
          <div>
            <button
              className="w-full bg-red-500 text-white p-4 rounded-md font-bold cursor-pointer"
              onClick={() => onClickDeleteEvent()}
            >このイベントを削除</button>
          </div>
        </div> : null}
      </div>

      {/* RSVP (join_late, leav_early) Modal */}
      <RSVPModal event={modalevent} isOpen={!!modalevent} closeModal={() => setModalEvent(null)} submit={submit} />

    </Layout>
  );
}

function getTimeLimitation(entry: Participation) {
  switch (entry.type) {
  case 'join_late':
    return <span className="ml-2 px-1 rounded bg-green-500 text-white text-sm">遅参 {entry.params?.time}</span>;
  case 'leave_early':
    return <span className="ml-2 px-1 rounded bg-green-500 text-white text-sm">{entry.params?.time} 早退</span>;
  default:
    return null;
  }
}

function PositionParticipationSection({ title, entries, join }: { title?: string, entries: Participation[], join: boolean }) {
  const coloring = (join ? (entries.length > 0 ? "bg-rose-900 text-white" : "bg-rose-200 text-red-800") : "bg-zinc-600 text-stone-100");
  if (entries.length == 0 && join == false) return <></>;
  return (
    <div key={title} className="mt-2 mb-4">
      <div className={"text-sm px-1 flex drop-shadow-md " + coloring}>
        <div className="flex-grow">{title == "OTHERS" ? "その他・不明な設定" : title}</div>
        <div>{entries.length}</div>
      </div>
      <div className="divide-y">
        {(entries.length == 0 && title) ?
          <span className="text-red-600">参加者なし</span> :
          entries.map((p, i) => <ParticipationRow key={i} entry={p} title={title == "OTHERS" ? "" : title} />)}
      </div>
    </div>
  );
}

function ParticipationRow({ entry, title }: { entry: Participation, title?: string }) {
  if (! entry.member) { console.log("member取得失敗", entry); return <></>; }
  const { member } = entry;
  const name = member.slack?.profile?.display_name || member.slack?.profile?.real_name;
  return (
    <div key={member.slack?.id} className="flex space-x-2 items-center">
      <div className="flex-auto">{name}</div>
      <div className="text-xs">{getTimeLimitation(entry)}</div>
      {title ? <></> : <div className="text-xs">{member.slack?.profile?.title}</div>}
    </div>
  );
}

function EndTime({end_time}) {
  if (!end_time) return <></>;
  const d = new Date(end_time);
  return <span className="text-md text-gray-800">~ {d.getHours()}:{("0" + d.getMinutes()).slice(-2)}</span>
}