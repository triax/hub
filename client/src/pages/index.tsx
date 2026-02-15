import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { EventList, EventRow } from "../../components/Events";
import { RSVPModal } from "../../components/Events/RSVPModal";
import Layout from "../../components/layout";
import TeamEvent from "../../models/TriaxEvent";
import type { EventTag } from "../../models/TriaxEvent";
import TeamEventRepo from "../../repository/EventRepo";
import { useAppContext } from "../context";

const repo = new TeamEventRepo();

type FilterKey = "練習" | "試合" | "イベント" | "その他";

const filters: { key: FilterKey; label: string; tags: EventTag[] }[] = [
  { key: "練習", label: "練習", tags: ["練習"] },
  { key: "試合", label: "試合", tags: ["試合"] },
  { key: "イベント", label: "イベント", tags: ["event"] },
  { key: "その他", label: "その他", tags: ["meeting", "UNKNOWN"] },
];

const defaultFilters: Set<FilterKey> = new Set(["練習", "試合"]);

function filterEvents(events: TeamEvent[], active: Set<FilterKey>): TeamEvent[] {
  const allowedTags = new Set<EventTag>();
  for (const f of filters) {
    if (active.has(f.key)) f.tags.forEach(t => allowedTags.add(t));
  }
  return events.filter(ev => allowedTags.has(ev.tag()));
}

export default function Top() {
  const { myself, startLoading, stopLoading } = useAppContext();
  const [modalevent, setModalEvent] = useState(null);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(new Set(defaultFilters));
  const navigate = useNavigate();
  useEffect(() => {
    repo.list().then(setEvents);
  }, []);
  const filteredEvents = useMemo(() => filterEvents(events, activeFilters), [events, activeFilters]);
  const toggleFilter = (key: FilterKey) => {
    setActiveFilters(prev => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };
  const submit = async function(params) {
    startLoading();
    const updated = await repo.rsvp(params);
    const newlist = events.map(ev => ev.google.id == updated.google.id ? updated : ev);
    setEvents(newlist);
    stopLoading();
  }
  return (
    <Layout>
      <div className="px-0 py-4 leading-6 text-lg font-medium text-gray-900 rounded-lg">
        <h1 role="heading">近日中の予定</h1>
      </div>
      <div className="flex gap-2 flex-wrap mb-3">
        {filters.map(f => (
          <button
            key={f.key}
            onClick={() => toggleFilter(f.key)}
            className={`px-3 py-1 text-sm font-medium rounded-full border transition-colors ${
              activeFilters.has(f.key)
                ? "bg-blue-500 text-white border-blue-500"
                : "bg-white text-gray-500 border-gray-300 hover:border-gray-400"
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>
      <EventList>
        {filteredEvents.map(event => <EventRow
          key={event.google.id} event={event} myself={myself}
          submit={submit}
          setModalEvent={setModalEvent}
          navigate={navigate}
        />)}
      </EventList>
      <RSVPModal event={modalevent} isOpen={!!modalevent} closeModal={() => setModalEvent(null)} submit={submit} />
    </Layout>
  );
}
