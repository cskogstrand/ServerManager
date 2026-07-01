<script setup lang="ts">
// Backup & restore. Backups are plain authenticated downloads; restore stages
// the uploaded database for the next boot (swapped in before the DB opens).
import { ref } from "vue";
import { csrfToken } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useServerStore } from "@/stores/server";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import AdminBackButton from "@/components/AdminBackButton.vue";

const toast = useToastStore();
const confirm = useConfirmStore();
const server = useServerStore();

const file = ref<File | null>(null);
const restoring = ref(false);

const downloads = [
  { href: "/api/server/smdata", label: "Database", hint: "smdata.db — all presets, events, users, instances", icon: "content" },
  { href: "/api/server/smcontent", label: "Content archive", hint: "smcontent.zip — imported tracks and cars", icon: "car" },
  { href: "/api/server/logfile", label: "Server log", hint: "logfile.log", icon: "terminal" },
];

function onFile(e: Event) {
  file.value = (e.target as HTMLInputElement).files?.[0] ?? null;
}

async function restore() {
  if (!file.value) {
    toast.error("Choose a database file first.");
    return;
  }
  if (server.instanceList.some((i) => i.running)) {
    toast.error("Stop every server before restoring.");
    return;
  }
  const ok = await confirm.ask({
    title: "Restore database",
    message: `Replace the current database with ${file.value.name}?`,
    detail:
      "It is staged and applied on the next restart. Your current database is kept as smdata.db.prev. This overwrites all presets, events, users and instances.",
    confirmLabel: "Stage restore",
    tone: "danger",
  });
  if (!ok) return;

  restoring.value = true;
  try {
    const data = new FormData();
    data.append("database", file.value);
    const res = await fetch("/api/maintenance/restore", {
      method: "POST",
      headers: { "X-CSRF-Token": csrfToken() },
      body: data,
    });
    const json = await res.json();
    if (!res.ok) {
      toast.error(json?.error?.message ?? "Restore failed.");
      return;
    }
    toast.success(json.message ?? "Restore staged. Restart to apply.", 12000);
    file.value = null;
  } catch (e) {
    toast.error(String(e));
  } finally {
    restoring.value = false;
  }
}
</script>

<template>
  <PageHeader title="Backup & Restore" subtitle="Download a full backup, or restore a database from a previous backup." icon="content">
    <template #prefix>
      <AdminBackButton />
    </template>
  </PageHeader>

  <div class="grid items-start gap-5 md:grid-cols-2">
    <Card title="Backup" class="min-w-0">
      <p class="mb-3 text-sm text-muted">Download the data you'd want to keep. The database holds everything except imported content files.</p>
      <div class="space-y-2">
        <a
          v-for="d in downloads"
          :key="d.href"
          :href="d.href"
          class="flex items-center gap-3 rounded-md border border-line bg-surface px-3 py-2.5 transition-colors hover:border-line-hi hover:bg-surface-2"
        >
          <Icon :name="d.icon" :size="18" class="shrink-0 text-accent" />
          <div class="min-w-0 flex-1">
            <div class="text-sm font-semibold">{{ d.label }}</div>
            <div class="truncate text-xs text-muted">{{ d.hint }}</div>
          </div>
          <Icon name="arrowDown" :size="16" class="shrink-0 text-dim" />
        </a>
      </div>
    </Card>

    <Card title="Restore database" class="min-w-0">
      <p class="mb-3 text-sm text-muted">
        Upload an <span class="font-mono">smdata.db</span> backup. It is staged and applied on the next restart;
        the current database is kept as <span class="font-mono">smdata.db.prev</span>.
      </p>
      <input
        type="file"
        accept=".db,.sqlite,.sqlite3"
        class="mb-3 w-full min-w-0 text-sm text-muted file:mr-3 file:rounded-md file:border file:border-line file:bg-surface-2 file:px-3 file:py-1.5 file:text-sm file:text-text"
        @change="onFile"
      />
      <p
        v-if="server.instanceList.some((i) => i.running)"
        class="mb-3 flex items-start gap-2 rounded-md border border-danger/40 bg-danger-glow px-3 py-2 text-sm text-danger"
      >
        <Icon name="alert" :size="16" class="mt-0.5 shrink-0" />
        Stop every running server before restoring.
      </p>
      <Button variant="danger" :disabled="restoring || !file" @click="restore">
        {{ restoring ? "Staging…" : "Stage restore" }}
      </Button>
    </Card>
  </div>
</template>
