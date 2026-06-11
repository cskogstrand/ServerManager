<script setup lang="ts">
// Phase 1 proof-of-pipeline dashboard: one live card per instance, fed by
// the SSE store. Start/stop wired through the typed API client.
// The full dashboard (queue, event editing, console) lands in Phase 5.
import { ref } from "vue";
import { useServerStore } from "@/stores/server";
import { ApiError } from "@/lib/api";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";

const server = useServerStore();
const busy = ref<Record<number, boolean>>({});
const error = ref("");

async function toggle(id: number, running: boolean) {
  busy.value[id] = true;
  error.value = "";
  try {
    await (running ? server.stop(id) : server.start(id));
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : String(e);
  } finally {
    busy.value[id] = false;
  }
}

function sessionLabel(type: number): string {
  return ["Booking", "Practice", "Qualify", "Race"][type] ?? "—";
}
</script>

<template>
  <h1 class="mb-5 text-xl font-bold">Dashboard</h1>

  <p v-if="error" class="mb-4 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger">
    {{ error }}
  </p>

  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
    <Card v-for="inst in server.instanceList" :key="inst.id">
      <template #header>
        <span
          class="size-2 rounded-full"
          :class="inst.running ? 'bg-ok' : 'bg-dim'"
        />
        <h2 class="text-sm font-semibold">{{ inst.name }}</h2>
        <span class="font-mono text-xs text-dim">:{{ inst.tcp_port }}</span>
      </template>
      <template #actions>
        <Button
          :variant="inst.running ? 'danger' : 'success'"
          size="sm"
          :disabled="busy[inst.id]"
          @click="toggle(inst.id, inst.running)"
        >
          {{ busy[inst.id] ? "…" : inst.running ? "Stop" : "Start" }}
        </Button>
      </template>

      <dl class="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
        <dt class="text-muted">Status</dt>
        <dd :class="inst.running ? 'text-ok' : 'text-dim'">
          {{ inst.running ? "Running" : "Stopped" }}
        </dd>
        <dt class="text-muted">Players</dt>
        <dd>{{ inst.players }}</dd>
        <template v-if="inst.running && inst.session">
          <dt class="text-muted">Session</dt>
          <dd>{{ sessionLabel(inst.session.type) }}</dd>
          <dt class="text-muted">Track</dt>
          <dd class="truncate font-mono text-xs">
            {{ inst.session.track || "—" }}
          </dd>
        </template>
      </dl>
    </Card>
  </div>

  <p v-if="server.loaded && server.instanceList.length === 0" class="text-muted">
    No server instances configured.
  </p>
</template>
