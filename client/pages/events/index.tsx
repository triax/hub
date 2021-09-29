import { useEffect, useState } from "react";
import { EventList, EventRow } from "../../components/Events";
import { RSVPModal } from "../../components/Events/RSVPModal";
import Layout from "../../components/layout";

// TODO: Repositoryつくれよ、いい加減
async function submitRSVP({event, answer, params}) {
  const endpoint = process.env.API_BASE_URL + "/api/1/events/answer";
  const res = await fetch(endpoint, { method: "POST", body: JSON.stringify({
    event: { id: event.google.id }, type: answer, params: params ? params : null,
  })});
  const result = await res.json();
  return result;
}

export default function Top(props) {
  const { myself, startLoading, stopLoading } = props;
  const [modalevent, setModalEvent] = useState(null);
  const [events, setEvents] = useState([]);
  useEffect(() => {
    fetch(process.env.API_BASE_URL + "/api/1/events") // TODO: Repository作れ
      .then(res => res.json())
      .then(evts => setEvents(evts));
  }, []);
  const submit = async function(params) {
    startLoading();
    const updated = await submitRSVP(params);
    const newlist = events.map(ev => ev.google.id == updated.google.id ? updated : ev);
    setEvents(newlist);
    stopLoading();
  }
  return (
    <Layout {...props} >
      <div className="px-0 py-4 leading-6 text-lg font-medium text-gray-900 rounded-lg">
        <h1>近日中の予定</h1>
      </div>
      <EventList>
        {events.map(event => <EventRow
          key={event.google.id} event={event} myself={myself}
          submit={submit}
          setModalEvent={setModalEvent}
        />)}
      </EventList>
      <RSVPModal event={modalevent} isOpen={!!modalevent} closeModal={() => setModalEvent(null)} submit={submit} />
    </Layout>
  );
}
