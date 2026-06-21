<script setup lang="ts">
// Guest Drivers: manage a roster of people who share one Assetto Corsa account
// (GUID), and — the operate-time core — pick who is behind the wheel of a live,
// connected car so their results land under the right name in leaderboards.
// Each guest links to its own profile page; past rows are reassigned from the
// Leaderboard itself (pencil on each row).
import { computed, onMounted, ref } from "vue";
import { ApiError } from "@/lib/api";
import {
  type GuestDriver,
  listGuestDrivers,
  createGuestDriver,
  updateGuestDriver,
  deleteGuestDriver,
  assignLiveDriver,
} from "@/lib/guestDriversApi";
import { fmtScore, timeAgo } from "@/lib/driversApi";
import { useServerStore, type DriverState } from "@/stores/server";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import PageHeader from "@/components/ui/PageHeader.vue";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Input from "@/components/ui/Input.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Select from "@/components/ui/Select.vue";
import Icon from "@/components/ui/Icon.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import DriverAvatar from "@/components/ui/DriverAvatar.vue";

const server = useServerStore();
const toast = useToastStore();
const confirm = useConfirmStore();

const roster = ref<GuestDriver[]>([]);
const busy = ref(false);

// --- roster form (add / edit) ----------------------------------------------
const editingId = ref<number | null>(null);
const formName = ref("");
const formNotes = ref("");
const editingName = computed(() => roster.value.find((d) => d.id === editingId.value)?.name ?? "");

async function guard(fn: () => Promise<void>) {
  busy.value = true;
  try {
    await fn();
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e));
  } finally {
    busy.value = false;
  }
}

async function load() {
  roster.value = await listGuestDrivers();
}

function resetForm() {
  editingId.value = null;
  formName.value = "";
  formNotes.value = "";
}

function startEdit(d: GuestDriver) {
  editingId.value = d.id;
  formName.value = d.name;
  formNotes.value = d.notes ?? "";
}

const save = () =>
  guard(async () => {
    const name = formName.value.trim();
    if (!name) {
      toast.error("A name is required.");
      return;
    }
    if (editingId.value) {
      await updateGuestDriver(editingId.value, name, formNotes.value.trim());
      toast.success(`Saved ${name}.`);
    } else {
      await createGuestDriver(name, formNotes.value.trim());
      toast.success(`Added ${name}.`);
    }
    resetForm();
    await load();
  });

const remove = (d: GuestDriver) =>
  guard(async () => {
    const ok = await confirm.ask({
      title: "Delete guest driver",
      message: `Delete ${d.name}?`,
      detail: "Leaderboard rows attributed to them revert to the account's own name. This cannot be undone.",
      confirmLabel: "Delete",
      tone: "danger",
    });
    if (!ok) return;
    await deleteGuestDriver(d.id);
    if (editingId.value === d.id) resetForm();
    toast.success(`Deleted ${d.name}.`);
    await load();
  });

// --- live cars (the "who is driving" panel) ---------------------------------
const rosterById = computed(() => {
  const m = new Map<number, GuestDriver>();
  for (const d of roster.value) m.set(d.id, d);
  return m;
});

// "— No guest driver —" (value 0) plus every roster entry.
const guestOptions = computed(() => [
  { value: 0, label: "— No guest driver —" },
  ...roster.value.map((d) => ({ value: d.id, label: d.name })),
]);

interface LiveCar {
  instanceId: number;
  instanceName: string;
  multiInstance: boolean;
  driver: DriverState;
}

const liveCars = computed<LiveCar[]>(() => {
  const running = server.instanceList.filter((i) => i.running);
  const out: LiveCar[] = [];
  for (const inst of running) {
    for (const d of inst.drivers) {
      if (!d.connected) continue;
      out.push({ instanceId: inst.id, instanceName: inst.name, multiInstance: running.length > 1, driver: d });
    }
  }
  return out;
});

// Name a live car is currently shown as: the assigned guest driver if any (and
// still on the roster), else the account's own AC name.
function liveDisplayName(d: DriverState): string {
  if (d.guest_driver_id) {
    const gd = rosterById.value.get(d.guest_driver_id);
    if (gd) return gd.name;
  }
  return d.name;
}

function isReassigned(d: DriverState): boolean {
  return !!d.guest_driver_id && rosterById.value.has(d.guest_driver_id);
}

const assignLive = (car: LiveCar, value: string | number) =>
  guard(async () => {
    const id = Number(value) || null;
    await assignLiveDriver(car.instanceId, car.driver.car_id, id);
    // The live roster refreshes over SSE; toast confirms the change landed.
    const target = id ? (rosterById.value.get(id)?.name ?? "guest driver") : car.driver.name;
    toast.success(id ? `${car.driver.name} now scores as ${target}.` : `Cleared — ${car.driver.name} scores under their own name.`);
  });

onMounted(() => guard(load));
</script>

<template>
  <PageHeader
    title="Guest Drivers"
    subtitle="One account, many drivers. Keep a roster of who shares an account, and pick who's behind the wheel so scores land under the right name."
    icon="users"
  />

  <!-- ░░ On track now — set who is driving live ░░ -->
  <Card class="mb-5">
    <template #header>
      <Icon name="play" :size="16" class="text-accent" />
      <h2 class="text-sm font-bold tracking-tight">On track now</h2>
      <span class="rounded-full border border-line bg-surface-2/70 px-2 py-0.5 font-mono text-[11px] text-dim">
        {{ liveCars.length }} connected
      </span>
    </template>

    <div v-if="liveCars.length" class="space-y-2">
      <div
        v-for="car in liveCars"
        :key="car.instanceId + ':' + car.driver.car_id"
        class="flex flex-wrap items-center gap-3 rounded-md border border-line bg-surface-2/40 p-3"
        :class="isReassigned(car.driver) ? 'border-accent/40' : ''"
      >
        <DriverAvatar :name="liveDisplayName(car.driver)" :guid="car.driver.guid" :size="40" />

        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <span class="truncate text-sm font-bold">{{ liveDisplayName(car.driver) }}</span>
            <span
              v-if="isReassigned(car.driver)"
              class="rounded-full border border-accent/40 bg-accent-dim px-1.5 py-0.5 text-[10px] font-bold tracking-wide text-accent uppercase"
            >
              Guest driver
            </span>
          </div>
          <div class="truncate font-mono text-[11px] text-dim">
            <span v-if="isReassigned(car.driver)">account: {{ car.driver.name }} · </span>
            {{ car.driver.car || "—" }}
            <span v-if="car.multiInstance"> · {{ car.instanceName }}</span>
            <span v-if="(car.driver.drift_best ?? 0) > 0"> · best {{ fmtScore(car.driver.drift_best ?? 0) }} pts</span>
          </div>
        </div>

        <label class="flex shrink-0 items-center gap-2">
          <span class="hidden text-[11px] font-bold tracking-wide text-muted uppercase sm:inline">Driving as</span>
          <Select
            class="!w-52"
            :model-value="car.driver.guest_driver_id ?? 0"
            :options="guestOptions"
            :disabled="busy"
            @update:model-value="(v) => assignLive(car, v ?? 0)"
          />
        </label>
      </div>
    </div>

    <EmptyState
      v-else
      icon="flag"
      title="Nobody on track"
      message="When a server is running and drivers are connected, they appear here so you can set who's actually driving."
    />
  </Card>

  <!-- ░░ Roster: list + add/edit ░░ -->
  <div class="grid items-start gap-5 lg:grid-cols-[1fr_340px]">
    <Card title="Roster" class="min-w-0">
      <ul v-if="roster.length" class="divide-y divide-line/60">
        <li v-for="d in roster" :key="d.id" class="flex items-center gap-3 py-2.5">
          <RouterLink
            :to="{ name: 'guest-driver-detail', params: { id: d.id } }"
            class="group flex min-w-0 flex-1 items-center gap-3"
            :title="`View ${d.name}'s profile`"
          >
            <DriverAvatar :name="d.name" :src="d.avatar_url" :size="34" />
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-semibold transition-colors group-hover:text-accent">{{ d.name }}</div>
              <div v-if="d.notes" class="truncate text-xs text-dim">{{ d.notes }}</div>
              <div v-else class="text-xs text-dim">Added {{ timeAgo(d.created_at) }}</div>
            </div>
          </RouterLink>
          <Button
            variant="dark"
            size="sm"
            :disabled="busy"
            :aria-label="`Edit ${d.name}`"
            @click="startEdit(d)"
          >
            <Icon name="edit" :size="14" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            :disabled="busy"
            :aria-label="`Delete ${d.name}`"
            @click="remove(d)"
          >
            <Icon name="trash" :size="14" />
          </Button>
        </li>
      </ul>

      <EmptyState
        v-else
        icon="users"
        title="No guest drivers yet"
        message="Add the people who share an account so you can attribute their drift runs and laps to them."
      />
    </Card>

    <Card :title="editingId ? `Edit ${editingName}` : 'Add guest driver'">
      <form class="space-y-1" @submit.prevent="save">
        <FormRow label="Name" for-id="gd-name" hint="Shown on leaderboards in place of the shared account name.">
          <Input id="gd-name" v-model="formName" placeholder="e.g. Kazuya Mori" />
        </FormRow>
        <FormRow label="Notes" for-id="gd-notes" hint="Optional — a reminder of who this is.">
          <Input id="gd-notes" v-model="formNotes" placeholder="e.g. shares the shop rig" />
        </FormRow>
        <div class="flex items-center gap-2 pt-1">
          <Button type="submit" :disabled="busy">
            <Icon :name="editingId ? 'check' : 'plus'" :size="15" />
            {{ editingId ? "Save" : "Add driver" }}
          </Button>
          <Button v-if="editingId" variant="ghost" :disabled="busy" @click="resetForm">Cancel</Button>
        </div>
      </form>
    </Card>
  </div>
</template>
