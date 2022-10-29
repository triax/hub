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

function memberSortFunc(prev: Participation, next: Participation): number {
  return prev.member?.slack?.profile?.title.toUpperCase() < next.member?.slack?.profile?.title.toUpperCase() ? -1 : 1;
}

export default function EventView(props) {
  const evrepo = useMemo(() => new TeamEventRepo(), []);
  const merepo = useMemo(() => new MemberCache(), []);
  const id = useRouter().query.id as string;
  const [event, setEvent] = useState<TeamEvent>(TeamEvent.placeholder());
  const [allMembers, setAllMembers] = useState<Member[]>([]);

  useEffect(() => {
    if (!id) return;
    evrepo.get(id).then(setEvent);
    merepo.list({cached: true}).then(setAllMembers)
  }, [id, evrepo, merepo]);

  if (!event || !event.google || !event.google.id) return <></>;
  if (allMembers.length == 0) return <></>;

  // 集計
  const sum: {
    yes: Participation[],
    no: Participation[],
    unanswered: Member[],
  } = Object.entries(event.participations).reduce((ctx, [id, entry]: [string, Participation]) => {
    if (['join', 'join_late', 'leave_early'].includes(entry.type)) ctx.yes.push({...entry, member: MemberCache.pick(id)});
    else ctx.no.push({ ...entry, member: MemberCache.pick(id) });
    ctx.unanswered = ctx.unanswered.filter(m => m.slack.id !== id);
    return ctx;
  }, { yes: [], no: [], unanswered: allMembers });

  // Sort
  sum.yes = sum.yes.sort(memberSortFunc);
  sum.no = sum.no.sort(memberSortFunc);

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

        <div className="py-4 space-y-6">

          <div>
            <div className="border-b">
              <span className="font-semibold">参加</span>
              <span className="px-4">{sum.yes.length}人</span>
            </div>
            <div className="divide-y">
              {sum.yes.map(p => <ParticipationRow key={p.member?.slack.id} entry={p} />)}
            </div>
          </div>

          <div>
            <div className="border-b">
              <span className="font-semibold">不参加</span>
              <span className="px-4">{sum.no.length}人</span>
            </div>
            <div className="divide-y">
              {sum.no.map(p => <ParticipationRow key={p.member?.slack.id} entry={p} />)}
            </div>
          </div>

          <div>
            <Disclosure>
              <Disclosure.Button as="div" className="border-b cursor-pointer">
                <span className="font-semibold">未回答</span>
                <span className="px-4">{sum.unanswered.length}人</span>
              </Disclosure.Button>
              <Disclosure.Panel as="div" className="divide-y">
                {sum.unanswered.map((m: any) => (
                  <div key={m.slack.id} className="flex space-x-2 items-center">
                    <div className="flex-auto">{m.slack.real_name}</div>
                    <div className="w-1/3 text-xs">ここにポジション表示</div>
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

function ParticipationRow({ entry }: { entry: Participation }) {
  if (! entry.member) { console.log("member取得失敗", entry); return <></>; }
  const { member } = entry;
  const name = member.slack?.profile?.display_name || member.slack?.profile?.real_name;
  return (
    <div key={member.slack?.id} className="flex space-x-2 items-center">
      <div className="flex-auto">{name}{getTimeLimitation(entry)}</div>
      <div className="w-1/3 text-xs">
        {member.slack?.profile?.title ? member.slack?.profile?.title : <span>
          Pos設定方法は
          <a href={process.env.HELP_PAGE_URL} target="_blank" rel="noreferrer"
            className="font-bold text-blue-500"
          >ここ</a>
        </span>}
      </div>
    </div>
  );
}

function EndTime({end_time}) {
  if (!end_time) return <></>;
  const d = new Date(end_time);
  return <span className="text-md text-gray-800">~ {d.getHours()}:{("0" + d.getMinutes()).slice(-2)}</span>
}