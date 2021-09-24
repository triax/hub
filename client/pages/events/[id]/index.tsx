import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import Layout from "../../../components/layout";
import { LocationMarkerIcon } from "@heroicons/react/outline";
import { Disclosure } from "@headlessui/react";
import { EventDateTime } from "../../../components/Events";

interface Participation {
  name: string;
  params?: any;
  type: string;
  title: string;
  picture: string;
}

export default function EventView(props) {
  const id = useRouter().query.id;
  const [event, setEvent] = useState(null);
  const [allMembers, setAllMembers] = useState([]);
  useEffect(() => {
    if (!id) return;
    // TODO: Repositoryつくる
    const base = process.env.API_BASE_URL;
    fetch(`${base}/api/1/events/${id}`).then(res => res.json()).then(res => setEvent(res));
    fetch(`${base}/api/1/members?cached=1`).then(res => res.json()).then(res => setAllMembers(res));
  }, [id]);
  if (!event) return <></>;
  const pats: Record<string, Participation> = JSON.parse(event.participations_json_str);
  const sum: Record<string, Participation[]> = Object.entries(pats).reduce((ctx, [id, entry]: [string, any]) => {
    if (['join', 'join_late', 'leave_early'].includes(entry.type)) ctx.yes.push(entry);
    else ctx.no.push(entry);
    ctx.unanswered = ctx.unanswered.filter(m => m.slack.id !== id);
    return ctx;
  }, { yes: [], no: [], unanswered: allMembers });
  sum.yes = sum.yes.sort((prev, next) => prev.title < next.title ? -1 : 1);
  sum.no = sum.no.sort((prev, next) => prev.title < next.title ? -1 : 1);
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
              {sum.yes.map((entry: any) => (
                <div key={entry.name} className="flex space-x-2 items-center">
                  <div className="flex-auto">{entry.name}</div>
                  <div className="w-1/3 text-xs">
                    {entry.title ? entry.title : "ここにポジション表示"}
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div>
            <div className="border-b">
              <span className="font-semibold">不参加</span>
              <span className="px-4">{sum.no.length}人</span>
            </div>
            <div className="divide-y">
              {sum.no.map((entry: any) => (
                <div key={entry.name} className="flex space-x-2 items-center">
                  <div className="flex-auto">{entry.name}</div>
                  <div className="w-1/3 text-xs">ここにポジション表示</div>
                </div>
              ))}
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
      </div>
    </Layout>
  );
}

function EndTime({end_time}) {
  if (!end_time) return <></>;
  const d = new Date(end_time);
  return <span className="text-md text-gray-800">~ {d.getHours()}:{("0" + d.getMinutes()).slice(-2)}</span>
}