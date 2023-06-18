function cn(...classes): string {
  return classes.filter(Boolean).join(' ');
}

export default function EventRSVPButtonsRow({
  event,
  answer,
  setModalEvent,
  submit,
}) {
  if (event.google.start_time < Date.now()) return null;
  return (
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
  );
}
