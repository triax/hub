import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import { EventList, EventRow } from "../../components/Events";
import { RSVPModal } from "../../components/Events/RSVPModal";
import Layout from "../../components/layout";
import TeamEvent from "../../models/TriaxEvent";
import TeamEventRepo from "../../repository/EventRepo";

const repo = new TeamEventRepo();

export default function Top(props) {
  const { myself, startLoading, stopLoading } = props;
  const [modalevent, setModalEvent] = useState(null);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  const router = useRouter();
  useEffect(() => {
    repo.list().then(setEvents);
  }, []);
  const submit = async function(params) {
    startLoading();
    const updated = await repo.rsvp(params);
    const newlist = events.map(ev => ev.google.id == updated.google.id ? updated : ev);
    setEvents(newlist);
    stopLoading();
  }
  return (
    <Layout {...props} >
      <div className="px-0 py-4 leading-6 text-lg font-medium text-gray-900 rounded-lg">
        <h1 role="heading">近日中の予定</h1>
      </div>
      <EventList>
        {events.map(event => <EventRow
          key={event.google.id} event={event} myself={myself}
          submit={submit}
          setModalEvent={setModalEvent}
          router={router}
        />)}
      </EventList>
      <RSVPModal event={modalevent} isOpen={!!modalevent} closeModal={() => setModalEvent(null)} submit={submit} />
    </Layout>
  );
}
