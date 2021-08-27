import { Dialog } from "@headlessui/react";
import { CheckCircleIcon } from "@heroicons/react/outline";
import { useEffect, useRef, useState } from "react";
import { EventDateTime } from "../components/DateTime";
import Layout from "../components/layout";

async function listEvents() {
  const endpoint = process.env.API_BASE_URL + "/api/1/events";
  const res = await fetch(endpoint);
  return res.json();
}

async function submitRSVP({event, answer, params}) {
  const endpoint = process.env.API_BASE_URL + "/api/1/events/answer";
  const res = await fetch(endpoint, { method: "POST", body: JSON.stringify({
    event: { id: event.google.id }, type: answer, params: params ? params : null,
  })});
  const result = await res.json();
  return result;
}

function cn(...classes): string {
  return classes.filter(Boolean).join(' ');
}

export default function Top(props) {
  const { myself, startLoading, stopLoading } = props;
  const [modalevent, setModalEvent] = useState(null);
  const [events, setEvents] = useState([]);
  useEffect(() => {
    setTimeout(() => { // FIXME: クソすぎる
      listEvents().then(ev => setEvents(ev));
    });
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
      <div className="divide-y grid grid-cols-1">
        {events.map(event => <EventRow
          key={event.google.id} event={event} myself={myself}
          submit={submit}
          setModalEvent={setModalEvent}
        />)}
      </div>
      <RSVPModal event={modalevent} isOpen={!!modalevent} closeModal={() => setModalEvent(null)} submit={submit} />
    </Layout>
  );
}

function EventLocation({location}) {
  return (
    <div>
      <div className="text-xs text-gray-400">
        {location}
      </div>
    </div>
  )
}

function EventRow({event, myself, submit, setModalEvent}) {
  const pats = JSON.parse(event.participations_json_str || "{}");
  const answer = pats[myself.openid.sub] || {};
  return (
    <div className="px-0 py-4">
      <EventDateTime timestamp={event.google.start_time} />
      <h3 className="text-gray-900 text-sm font-bold">{event.google.title}</h3>
      <EventLocation location={event.google.location} />
      <EventParticipantsIcons pats={pats} onClick={() => location.href = `/events/${event.google.id.replace(/@google\.com$/, "")}`} />
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
            <div className="w-1/3 flex justify-center cursor-pointer"
              onClick={() => setModalEvent(event)}
            >
              <span className={cn(
                'px-1 py-1 rounded-md',
                ['join_late', 'leave_early'].includes(answer.type) ? 'bg-green-400 text-white' : ''
              )}>遅参/早退</span>
            </div>
            <div className="w-1/3 flex justify-center cursor-pointer"
              onClick={() => submit({ event, answer: "absent" })}
            >
              <span className={cn(
                'px-1 py-1 rounded-md',
                answer.type == 'absent' ? 'bg-red-400 text-white' : ''
              )}>不参加</span>
            </div>
            <div className="w-1/3 flex justify-center cursor-pointer"
              onClick={() => submit({ event, answer: "join" })}
            >
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

function RSVPModal({
  event,
  isOpen,
  closeModal,
  submit,
}) {
  const [ptype, setPType] = useState("leave_early");
  if (!event) return <></>;
  const defaultTime = ((d) => ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2))(new Date(event.google.start_time));
  return (
    <Dialog
      open={isOpen}
      as="div"
      className="fixed inset-0 z-10 overflow-y-auto"
      onClose={closeModal}
    >
      <div className="min-h-screen px-4 text-center">
        <Dialog.Overlay className="fixed inset-0 bg-black bg-opacity-40" />

        {/* This element is to trick the browser into centering the modal contents. */}
        <span className="inline-block h-screen align-middle" aria-hidden="true">&#8203;</span>

        <div
          className="inline-block w-full max-w-md p-6 my-8 overflow-hidden text-left align-middle transition-all transform bg-white shadow-xl rounded-2xl">
          <EventDateTime timestamp={event.google.start_time} />
          <Dialog.Title as="h3" className="text-lg font-medium leading-6 text-gray-900">{event.google.title}</Dialog.Title>
          <EventLocation location={event.google.location} />
          <ParticipationSelectBoxes
            ptype={ptype} defaultTime={defaultTime} setPType={setPType}
            onCancel={closeModal} onCommit={async ({type, params}) => {
              await submit({event, answer: type, params});
              closeModal();
            }}
          />
        </div>
      </div>
    </Dialog>
  )
}

function ParticipationSelectBoxes({ptype, setPType, defaultTime, onCancel, onCommit}) {
  const refs = {
    "leave_early": useRef<HTMLInputElement>(),
    "join_late": useRef<HTMLInputElement>(),
  };
  return (
    <>
      <div className="mt-2 flex-row">
        <div
          className={cn(
            "cursor-pointer flex border border-gray-200 px-4 py-4 rounded-md mb-2",
            ptype == "leave_early" ? "bg-blue-100" : "",
          )}
          onClick={() => setPType("leave_early")}
        >
          <div className="flex w-12 justify-center">
            {ptype == "leave_early" ? <CheckCircleIcon color="gray" width={24} /> : null}
          </div>
          <div className="flex flex-auto items-center">
            <span>早退</span>
          </div>
          <div>
            <input type="time" className="form-input" defaultValue={defaultTime} ref={refs.leave_early} /> 頃
          </div>
        </div>
        <div
          className={cn(
            "cursor-pointer flex border border-gray-200 px-4 py-4 rounded-md",
            ptype == "join_late" ? "bg-blue-100" : "",
          )}
          onClick={() => setPType("join_late")}
        >
          <div className="flex w-12 justify-center px-2">
            {ptype == "join_late" ? <CheckCircleIcon color="gray" width={24} /> : null}
          </div>
          <div className="flex flex-auto items-center">
            <span>遅参</span>
          </div>
          <div>
            <input type="time" className="form-input" defaultValue={defaultTime} ref={refs.join_late} /> 頃
          </div>
        </div>
      </div>

      <div className="mt-4">
        <button
          type="button"
          className="inline-flex justify-center px-4 py-2 text-sm font-medium text-gray-600 bg-white border hover:border-gray-400 rounded-md hover:bg-gray-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-blue-500 mr-2"
          onClick={onCancel}
        >キャンセル</button>
        <button
          type="button"
          className="inline-flex justify-center px-4 py-2 text-sm font-medium text-blue-900 bg-blue-100 border border-transparent rounded-md hover:bg-blue-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-blue-500"
          onClick={() => onCommit({ type: ptype, params: { time: refs[ptype].current.value } })}
        >これでよし</button>
      </div>

    </>
  );
}

function EventParticipantsIcons({pats, onClick = () => {}}) {
  const entries = Object.entries(pats)
    .filter(([_, p]: [string, any]) => p.type == 'join' || p.type == 'join_late') || [];
  if (entries.length == 0) return null;
  const maxVisible = 10;
  const visibles = entries.length > maxVisible ? entries.slice(0, maxVisible) : entries;
  const rest = entries.length - visibles.length;
  return (
    <div className="flex" onClick={onClick} >
      <div className="flex -space-x-2">
        {visibles.map(([id, p]: [string, any]) => (
          <img
            key={id} src={p.picture} alt={p.name}
            className="w-6 h-6 rounded-full border-2 border-white"
          />
        ))}
      </div>
      {rest > 0 ? <span className="text-gray-400 text-sm items-center">+{rest}</span> : null}
    </div>
  )
}