import { onMounted, ref, type Ref } from "vue";
import { api, ApiError } from "@/lib/api";
import { useQueryParam, numberParam } from "@/lib/useQueryParam";
import { useConfirmStore } from "@/stores/confirm";
import type { presetResource } from "@/lib/presets";
import type { DropDownList } from "@/types/generated";

type UsageMap = Record<string, Record<string, number>>;

// Common controller for the four preset pages: list + select + create +
// delete + save, with notice/error handling.
export function usePresetPage<T extends { id?: number }>(
  resource: ReturnType<typeof presetResource<T>>,
  prepare?: (form: T) => T,
) {
  const items: Ref<DropDownList[]> = ref([]);
  // Selected preset mirrored to ?sel so refresh/back reopens the same one.
  const selectedId = useQueryParam<number | null>("sel", null, numberParam());
  const form = ref<T | null>(null) as Ref<T | null>;
  const busy = ref(false);
  const notice = ref("");
  const error = ref("");
  const confirm = useConfirmStore();

  // How many events use each preset of this kind: { presetId: count }.
  const usage = ref<Record<string, number>>({});
  const usedBy = (id: number | null) => (id == null ? 0 : (usage.value[String(id)] ?? 0));

  async function reloadUsage() {
    try {
      const all = await api.get<UsageMap>("/api/presets/usage");
      usage.value = all[resource.plural] ?? {};
    } catch {
      usage.value = {};
    }
  }

  // Snapshot of the form as last loaded/saved, for unsaved-changes detection.
  let baseline = "";
  const snapshot = () => (form.value ? JSON.stringify(form.value) : "");
  const markClean = () => (baseline = snapshot());
  const isDirty = () => form.value !== null && snapshot() !== baseline;

  async function guard(fn: () => Promise<void>) {
    busy.value = true;
    notice.value = "";
    error.value = "";
    try {
      await fn();
    } catch (e) {
      error.value = e instanceof ApiError ? e.message : String(e);
    } finally {
      busy.value = false;
    }
  }

  const reloadList = async () => {
    items.value = await resource.list();
  };

  const select = (id: number) =>
    guard(async () => {
      const data = await resource.get(id);
      form.value = prepare ? prepare(data) : data;
      selectedId.value = id;
      markClean();
    });

  const create = (name: string) =>
    guard(async () => {
      const id = await resource.create(name);
      await reloadList();
      await select(id);
    });

  const remove = (id: number) =>
    guard(async () => {
      const ok = await confirm.ask({
        title: "Delete preset",
        message: "Delete this preset?",
        detail: "Events using it will block the delete.",
        confirmLabel: "Delete preset",
        tone: "danger",
      });
      if (!ok) return;
      await resource.remove(id);
      if (selectedId.value === id) {
        selectedId.value = null;
        form.value = null;
      }
      await Promise.all([reloadList(), reloadUsage()]);
    });

  const save = () =>
    guard(async () => {
      if (!form.value || selectedId.value === null) return;
      await resource.update(selectedId.value, form.value);
      await reloadList();
      markClean();
      notice.value = "Saved.";
    });

  // Clone-before-edit: copy a preset and switch to the new one so a shared
  // preset can be changed without affecting the events already using the source.
  const duplicate = (id: number) =>
    guard(async () => {
      const src = await resource.get(id);
      const baseName = items.value.find((i) => i.id === id)?.name ?? "Preset";
      const copyName = `${baseName} copy`;
      const newId = await resource.create(copyName);
      await resource.update(newId, { ...src, id: newId, name: copyName } as T);
      await Promise.all([reloadList(), reloadUsage()]);
      await select(newId);
      notice.value = "Duplicated — you're editing the copy.";
    });

  onMounted(() =>
    guard(async () => {
      await Promise.all([reloadList(), reloadUsage()]);
      // Restore the preset named in ?sel (if it still exists).
      if (selectedId.value != null && items.value.some((i) => i.id === selectedId.value)) {
        const data = await resource.get(selectedId.value);
        form.value = prepare ? prepare(data) : data;
        markClean();
      } else if (selectedId.value != null) {
        selectedId.value = null;
      }
    }),
  );

  return { items, selectedId, form, busy, notice, error, usage, usedBy, select, create, remove, duplicate, save, isDirty };
}
