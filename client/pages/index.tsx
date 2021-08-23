import { useEffect, useState } from "react";
import Layout from "../components/layout";

async function listEvents() {
  const endpoint = process.env.API_BASE_URL + "/api/1/events";
  const res = await fetch(endpoint);
  return res.json();
}

async function submitRSVP({event, answer}) {
  const endpoint = process.env.API_BASE_URL + "/api/1/events/answer";
  const res = await fetch(endpoint, { method: "POST", body: JSON.stringify({
    event: { id: event.google.id }, type: answer, params: null,
  })});
  const result = await res.json();
  // console.log(res.status, res.statusText, JSON.parse(result.participations_json_str));
  return result;
}

function cn(...classes): string {
  return classes.filter(Boolean).join(' ');
}

export default function Top(props) {
  const { myself } = props;
  const [events, setEvents] = useState([]);
  useEffect(() => {
    setTimeout(() => { // FIXME: クソすぎる
      listEvents().then(ev => setEvents(ev));
    });
  }, []);
  const submit = async function(params) {
    const updated = await submitRSVP(params);
    const newlist = events.map(ev => ev.google.id == updated.google.id ? updated : ev);
    setEvents(newlist);
  }
  return (
    <Layout {...props} >
      <div className="px-0 py-4 leading-6 text-lg font-medium text-gray-900 rounded-lg">
        <h1>近日中の予定</h1>
      </div>
      <div className="divide-y grid grid-cols-1">
        {events.map(event => <EventRow key={event.google.id} event={event} myself={myself} submit={submit} />)}
      </div>
    </Layout>
  );
}

const weekday = {
  0: "日", 1: "月", 2: "火", 3: "水", 4: "木", 5: "金", 6: "土"
}

function EventRow({event, myself, submit}) {
  const date = new Date(event.google.start_time);
  const pats = JSON.parse(event.participations_json_str || "{}");
  const answer = pats[myself.sub] || {};
  return (
    <div className="px-0 py-4">
      <div className="text-xs text-gray-500">
        {date.getMonth() + 1}月 {date.getDate()}日（{weekday[date.getDay()]}） {date.getHours()}:{("0" + date.getMinutes()).slice(-2)}
      </div>
      <h3 className="text-gray-900 text-sm font-bold">{event.google.title}</h3>
      <div>
        <div className="text-xs text-gray-400">
          {event.google.location}
        </div>
      </div>
      <div className="px-0 pt-4 flex items-center">
        <div className="flex">
          {answer.type === undefined ? (
            <div
              className="text-red-600 font-medium text-sm border border-red-600 px-1 py-1 rounded-md"
            ><span>未回答</span></div>
          ) : null}
        </div>
        <div className="flex flex-grow flex-row-reverse">
          <div className="w-60 flex justify-end divide-x font-medium text-gray-400">
            <div className="w-1/3 flex justify-center">
              <span className={cn(
                'px-1 py-1',
                ['join_late', 'leave_early'].includes(answer.type) ? 'bg-red-400 text-white' : ''
              )}>遅参/早退</span>
            </div>
            <div className="w-1/3 flex justify-center" onClick={() => submit({ event, answer: "absent" })}>
              <span className={cn(
                'px-1 py-1 rounded-md',
                answer.type == 'absent' ? 'bg-red-400 text-white' : ''
              )}>不参加</span>
            </div>
            <div className="w-1/3 flex justify-center" onClick={() => submit({ event, answer: "join" })}>
              <span className={cn(
                'px-3 py-1 rounded-md',
                answer.type == 'join' ? 'bg-blue-600 text-white' : ''
              )}>参加</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}