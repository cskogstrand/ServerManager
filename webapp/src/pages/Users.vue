<script setup lang="ts">
// Admin-only user management: list, create, change role, reset password,
// delete. Backend (RoleMiddleware) is the real gate; this is the UI for it.
import { onMounted, ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useToastStore } from "@/stores/toast";
import { useConfirmStore } from "@/stores/confirm";
import { useAuthStore } from "@/stores/auth";
import Card from "@/components/ui/Card.vue";
import Button from "@/components/ui/Button.vue";
import FormRow from "@/components/ui/FormRow.vue";
import Input from "@/components/ui/Input.vue";
import Select from "@/components/ui/Select.vue";
import Icon from "@/components/ui/Icon.vue";
import PageHeader from "@/components/ui/PageHeader.vue";

interface UserRow {
  name: string;
  role: string;
}

const toast = useToastStore();
const confirm = useConfirmStore();
const auth = useAuthStore();

const users = ref<UserRow[]>([]);
const busy = ref(false);

const roleOptions = [
  { value: "admin", label: "Admin — full access" },
  { value: "steward", label: "Steward — operate servers & queues" },
  { value: "viewer", label: "Viewer — read only" },
];

const newName = ref("");
const newPassword = ref("");
const newRole = ref("steward");

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
  users.value = (await api.get<{ items: UserRow[] }>("/api/users")).items;
}

const createUser = () =>
  guard(async () => {
    if (!newName.value.trim() || !newPassword.value) {
      toast.error("Name and password are required.");
      return;
    }
    await api.post("/api/users", { name: newName.value.trim(), password: newPassword.value, role: newRole.value });
    toast.success(`User ${newName.value.trim()} created.`);
    newName.value = "";
    newPassword.value = "";
    newRole.value = "steward";
    await load();
  });

const setRole = (u: UserRow, role: string) =>
  guard(async () => {
    await api.put(`/api/users/${encodeURIComponent(u.name)}/role`, { role });
    toast.success(`${u.name} is now ${role}.`);
    await load();
  });

const resetPassword = (u: UserRow) =>
  guard(async () => {
    const pw = window.prompt(`New password for ${u.name}:`);
    if (!pw) return;
    await api.put(`/api/users/${encodeURIComponent(u.name)}/password`, { password: pw });
    toast.success(`Password reset for ${u.name}.`);
  });

const removeUser = (u: UserRow) =>
  guard(async () => {
    const ok = await confirm.ask({
      title: "Delete user",
      message: `Delete ${u.name}?`,
      detail: "They lose access immediately. This cannot be undone.",
      confirmLabel: "Delete user",
      tone: "danger",
    });
    if (!ok) return;
    await api.delete(`/api/users/${encodeURIComponent(u.name)}`);
    toast.success(`User ${u.name} deleted.`);
    await load();
  });

onMounted(() => guard(load));
</script>

<template>
  <PageHeader title="Users & Roles" subtitle="Manage who can sign in and what they can do." icon="users" />

  <div class="grid items-start gap-5 lg:grid-cols-[1fr_320px]">
    <Card title="Users">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-line text-left text-xs tracking-wide text-muted uppercase">
            <th class="py-2 pr-2 font-semibold">Name</th>
            <th class="py-2 pr-2 font-semibold">Role</th>
            <th class="py-2 font-semibold"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.name" class="border-b border-line/60">
            <td class="py-2 pr-2 font-medium">
              {{ u.name }}
              <span v-if="u.name === auth.user?.name" class="text-xs text-dim">(you)</span>
            </td>
            <td class="py-2 pr-2">
              <Select
                :model-value="u.role"
                class="!w-44"
                :options="roleOptions"
                @update:model-value="(r) => setRole(u, String(r))"
              />
            </td>
            <td class="py-2 text-right whitespace-nowrap">
              <Button variant="dark" size="sm" :disabled="busy" @click="resetPassword(u)">Reset password</Button>
              <Button
                v-if="u.name !== auth.user?.name"
                variant="ghost"
                size="sm"
                class="ml-1"
                aria-label="Delete user"
                :disabled="busy"
                @click="removeUser(u)"
              >
                <Icon name="trash" :size="14" />
              </Button>
            </td>
          </tr>
        </tbody>
      </table>
    </Card>

    <Card title="Add user">
      <FormRow label="Name" for-id="uname">
        <Input id="uname" v-model="newName" />
      </FormRow>
      <FormRow label="Password" for-id="upass">
        <Input id="upass" v-model="newPassword" type="password" />
      </FormRow>
      <FormRow label="Role" for-id="urole">
        <Select id="urole" v-model="newRole" :options="roleOptions" />
      </FormRow>
      <Button class="mt-1" :disabled="busy" @click="createUser">
        <Icon name="plus" :size="15" />
        Create user
      </Button>
    </Card>
  </div>
</template>
