export function isTapingManager(myself: any): boolean {
  if (!myself?.slack?.id || myself.slack.id === "xxx") return false;
  if (myself.slack.is_admin) return true;
  return !!(myself.slack.profile?.title?.match(/trainer|staff/i));
}
