<script setup lang="ts">
import { onMounted, ref } from "vue";
import { api } from "@/lib/api";
import Card from "@/components/ui/Card.vue";

interface AboutInfo {
  version: string;
  config_folder: string;
  temp_folder: string;
}

const info = ref<AboutInfo | null>(null);

onMounted(async () => {
  info.value = await api.get<AboutInfo>("/api/about");
});

const downloads = [
  { href: "/api/server/logfile", label: "logfile.log" },
  { href: "/api/server/smcontent", label: "smcontent.zip" },
  { href: "/api/server/smdata", label: "smdata.db" },
];
</script>

<template>
  <h1 class="mb-5 text-xl font-bold">About</h1>

  <div class="max-w-xl space-y-4">
    <Card title="Server Manager">
      <dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
        <dt class="text-muted">Version</dt>
        <dd class="font-mono">{{ info?.version ?? "…" }}</dd>
        <dt class="text-muted">Config folder</dt>
        <dd class="font-mono break-all">{{ info?.config_folder ?? "…" }}</dd>
        <dt class="text-muted">Temp folder</dt>
        <dd class="font-mono break-all">{{ info?.temp_folder ?? "…" }}</dd>
      </dl>
    </Card>

    <Card title="Downloads">
      <div class="flex flex-wrap gap-2">
        <a
          v-for="d in downloads"
          :key="d.href"
          :href="d.href"
          class="rounded-md border border-line bg-surface-2 px-3 py-1.5 text-sm text-muted hover:border-line-hi hover:text-text"
        >
          ⬇ {{ d.label }}
        </a>
      </div>
    </Card>
  </div>
</template>
