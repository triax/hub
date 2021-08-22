import { useEffect, useState } from "react";
import Layout from "../components/layout";

async function listEvents() {
  const endpoint = process.env.API_BASE_URL + "/api/1/events";
  const res = await fetch(endpoint, {
    cache: "no-cache",
  });
  return res.json();
}


export default function Top(props) {
  const [events, setEvents] = useState([]);
  useEffect(() => {
    setTimeout(() => { // FIXME: クソすぎる
      listEvents().then(ev => setEvents(ev));
    });
  }, []);
  const { myself } = props;
  return (
    <Layout {...props} >
      <div className="px-0 py-4 leading-6 text-lg font-medium text-gray-900 rounded-lg">
        <h1>近日中の予定</h1>
      </div>
      <div className="divide-y grid grid-cols-1">
        {events.map(event => <EventRow key={event.google.id} event={event} myself={myself} />)}
      </div>
    </Layout>
  );
}

const weekday = {
  0: "日", 1: "月", 2: "火", 3: "水", 4: "木", 5: "金", 6: "土"
}

function EventRow({event, myself}) {
  const date = new Date(event.google.start_time);
  const answer = (event.participations || {})[myself.sub];
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
      <div className="px-0 pt-4 flex flex-shrink-0 items-center">
        {answer === undefined ? (
          <div
            className="text-red-600 font-medium text-sm border border-red-600 px-1 py-1 rounded-md"
          ><span>未回答</span></div>
        ) : null}
        <div className="flex-grow flex justify-end divide-x font-medium text-gray-900">
          <div className="px-2">早退/遅参</div>
          <div className="px-2">不参加</div>
          <div className="px-2">参加</div>
        </div>
      </div>
    </div>
  );
}