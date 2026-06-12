<script setup lang="ts">
// Server Setup: one place to see whether the server can actually run, with a
// link from every failed check to the editor that fixes it. Pulls aggregated
// facts from /api/server/readiness and turns them into pass/fail rows.
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import Skeleton from "@/components/ui/Skeleton.vue";

interface Readiness {
  install_path: string;
  acserver_found: boolean;
  cfg_filled: boolean;
  mod_filled: boolean;
  content: { tracks: number; cars: number; weathers: number };
  presets: { difficulties: number; sessions: number; classes: number; times: number };
  events: number;
  instances: { id: number; name: string; run_mode: string; queue_pending: number; is_running: boolean }[];
  port_conflict: string;
}

const toast = useToastStore();
const data = ref<Readiness | null>(null);
const loading = ref(true);

async function load() {
  loading.value = true;
  try {
    data.value = await api.get<Readiness>("/api/server/readiness");
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    loading.value = false;
  }
}

onMounted(load);

interface Check {
  label: string;
  ok: boolean;
  detail: string;
  to: string;
  cta: string;
}

const checks = computed<Check[]>(() => {
  const d = data.value;
  if (!d) return [];
  const filledPresets =
    d.presets.difficulties > 0 && d.presets.sessions > 0 && d.presets.classes > 0 && d.presets.times > 0;
  const anyQueued = d.instances.some((i) => i.queue_pending > 0 || i.run_mode === "repeat_event");
  return [
    {
      label: "Assetto Corsa install path",
      ok: d.acserver_found,
      detail: d.acserver_found
        ? `acServer found under ${d.install_path}`
        : "No acServer binary found. Set the install path so the server can launch.",
      to: "/content",
      cta: "Set install path",
    },
    {
      label: "Server configuration",
      ok: d.cfg_filled,
      detail: d.cfg_filled ? "Base server config saved." : "Save the base server configuration (name, ports, limits).",
      to: "/settings",
      cta: "Open configuration",
    },
    {
      label: "Content cache",
      ok: d.content.tracks > 0 && d.content.cars > 0,
      detail:
        d.content.tracks > 0 && d.content.cars > 0
          ? `${d.content.tracks} tracks · ${d.content.cars} cars · ${d.content.weathers} weather`
          : "No tracks or cars cached. Import content and rebuild the cache.",
      to: "/content",
      cta: "Manage content",
    },
    {
      label: "Presets",
      ok: filledPresets,
      detail: filledPresets
        ? `${d.presets.classes} classes · ${d.presets.sessions} sessions · ${d.presets.times} time · ${d.presets.difficulties} difficulty`
        : "Create at least one filled preset of each type before building an event.",
      to: "/presets/classes",
      cta: "Build presets",
    },
    {
      label: "Runnable events",
      ok: d.events > 0,
      detail: d.events > 0 ? `${d.events} event${d.events === 1 ? "" : "s"} defined.` : "No events yet. Build one to queue or repeat.",
      to: "/events",
      cta: "Build an event",
    },
    {
      label: "Instance ports",
      ok: d.port_conflict === "",
      detail: d.port_conflict === "" ? `${d.instances.length} instance${d.instances.length === 1 ? "" : "s"}, no port conflicts.` : d.port_conflict,
      to: "/settings/instances",
      cta: "Fix instances",
    },
    {
      label: "Something queued to run",
      ok: anyQueued,
      detail: anyQueued ? "An event is queued or set to repeat." : "Queue an event or set an instance to repeat one.",
      to: "/queue",
      cta: "Open queue",
    },
  ];
});

const ready = computed(() => checks.value.length > 0 && checks.value.every((c) => c.ok));
const passing = computed(() => checks.value.filter((c) => c.ok).length);
</script>

<template>
  <PageHeader
    title="Server Setup"
    subtitle="Everything needed before a server can run, in one place. Each failing check links to where you fix it."
    icon="settings"
  >
    <template #actions>
      <Button variant="dark" size="sm" :disabled="loading" @click="load">
        <Icon name="activity" :size="15" />
        Re-check
      </Button>
    </template>
  </PageHeader>

  <div v-if="loading" class="space-y-3">
    <Skeleton v-for="n in 6" :key="n" class="h-16" />
  </div>

  <template v-else-if="data">
    <Card
      class="mb-4"
      :class="ready ? 'border-ok/40' : 'border-accent/40'"
    >
      <div class="flex items-center gap-3">
        <div
          class="grid size-11 shrink-0 place-items-center rounded-full"
          :class="ready ? 'bg-ok-glow text-ok' : 'bg-accent-dim text-accent'"
        >
          <Icon :name="ready ? 'check' : 'alert'" :size="22" />
        </div>
        <div>
          <h2 class="text-sm font-bold">{{ ready ? "Ready to race" : "Setup incomplete" }}</h2>
          <p class="text-sm text-muted">{{ passing }} of {{ checks.length }} checks passing.</p>
        </div>
      </div>
    </Card>

    <div class="space-y-2">
      <div
        v-for="c in checks"
        :key="c.label"
        class="flex items-center gap-3 rounded-md border border-line bg-surface px-3 py-2.5"
      >
        <Icon
          :name="c.ok ? 'check' : 'alert'"
          :size="18"
          :class="c.ok ? 'shrink-0 text-ok' : 'shrink-0 text-danger'"
        />
        <div class="min-w-0 flex-1">
          <div class="text-sm font-semibold">{{ c.label }}</div>
          <div class="truncate text-xs text-muted">{{ c.detail }}</div>
        </div>
        <RouterLink v-if="!c.ok" :to="c.to" class="shrink-0">
          <Button variant="dark" size="sm">{{ c.cta }}</Button>
        </RouterLink>
      </div>
    </div>
  </template>
</template>
