import { Dialog, DialogBackdrop, DialogPanel, DialogTitle } from "@headlessui/react";
import { CheckCircleIcon } from "@heroicons/react/outline";
import { useRef, useState } from "react";
import { EventDateTime, EventLocation } from "..";

function cn(...classes): string {
  return classes.filter(Boolean).join(' ');
}

export function RSVPModal({
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
      className="relative z-10"
      onClose={closeModal}
    >
      {/* 背景クリック・Esc での close は Dialog/DialogPanel が native に処理する */}
      <DialogBackdrop className="fixed inset-0 bg-black/40" />
      <div className="fixed inset-0 overflow-y-auto">
        <div className="flex min-h-full items-center justify-center p-4 text-center">
          <DialogPanel
            className="w-full max-w-md p-6 overflow-hidden text-left align-middle transition-all transform bg-white shadow-xl rounded-2xl">
            <EventDateTime timestamp={event.google.start_time} />
            <DialogTitle as="h3" className="text-lg font-medium leading-6 text-gray-900">{event.google.title}</DialogTitle>
            <EventLocation location={event.google.location} />
            <ParticipationSelectBoxes
              ptype={ptype} defaultTime={defaultTime} setPType={setPType}
              onCancel={closeModal} onCommit={async ({type, params}) => {
                await submit({event, answer: type, params});
                closeModal();
              }}
            />
          </DialogPanel>
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

