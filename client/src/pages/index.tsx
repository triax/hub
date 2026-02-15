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

type ScheduleTab = "all" | "練習" | "試合" | "イベント" | "その他";

const tabs: { key: ScheduleTab; label: string }[] = [
  { key: "all", label: "全て" },
  { key: "練習", label: "練習" },
  { key: "試合", label: "試合" },
  { key: "イベント", label: "イベント" },
  { key: "その他", label: "その他" },
];

function filterByTab(events: TeamEvent[], tab: ScheduleTab): TeamEvent[] {
  if (tab === "all") return events;
  return events.filter(ev => {
    const tag = ev.tag();
    switch (tab) {
      case "練習": return tag === "練習";
      case "試合": return tag === "試合";
      case "イベント": return tag === "event";
      case "その他": return tag === "meeting" || tag === "UNKNOWN";
    }
  });
}

export default function Top() {
  const { myself, startLoading, stopLoading } = useAppContext();
  const [modalevent, setModalEvent] = useState(null);
  const [events, setEvents] = useState<TeamEvent[]>([]);
  const [activeTab, setActiveTab] = useState<ScheduleTab>("all");
  const navigate = useNavigate();
  useEffect(() => {
    repo.list().then(setEvents);
  }, []);
  const filteredEvents = useMemo(() => filterByTab(events, activeTab), [events, activeTab]);
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
      <div className="flex gap-1 overflow-x-auto border-b border-gray-200 mb-2">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-3 py-2 text-sm font-medium whitespace-nowrap border-b-2 transition-colors ${
              activeTab === tab.key
                ? "border-blue-500 text-blue-600"
                : "border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300"
            }`}
          >
            {tab.label}
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
