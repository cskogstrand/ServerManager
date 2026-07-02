import { api } from "@/lib/api";
import { useConfirmStore } from "@/stores/confirm";
import { useServerStore, type InstanceState } from "@/stores/server";

const fieldByTemplate: Record<string, string> = {
  classes: "class_id",
  sessions: "session_id",
  times: "time_id",
  difficulties: "difficulty_id",
  "drift-scoring-modes": "drift_scoring_mode_id",
};

interface ServerStatus {
  current_event?: Record<string, unknown>;
}

export async function restartServersUsingTemplate(template: string, id: number): Promise<number> {
  const field = fieldByTemplate[template];
  if (!field) return 0;

  const server = useServerStore();
  const confirm = useConfirmStore();
  await server.load();

  const matches: InstanceState[] = [];
  for (const inst of server.instanceList.filter((i) => i.running)) {
    const status = await api.get<ServerStatus>(`/api/server/status?instance=${inst.id}`);
    if (Number(status.current_event?.[field] ?? 0) === id) matches.push(inst);
  }
  if (!matches.length) return 0;

  const ok = await confirm.ask({
    title: "Restart server now?",
    message:
      matches.length === 1
        ? `${matches[0].name} is using this template. Restart it now?`
        : `${matches.length} running servers are using this template. Restart them now?`,
    detail: "Connected players are disconnected while the current event reloads.",
    confirmLabel: matches.length === 1 ? "Restart server now" : `Restart ${matches.length} servers now`,
    cancelLabel: "Later",
    tone: "danger",
  });
  if (!ok) return 0;

  for (const inst of matches) {
    await api.post(`/api/server/restart?instance=${inst.id}`);
  }
  await server.load();
  return matches.length;
}
