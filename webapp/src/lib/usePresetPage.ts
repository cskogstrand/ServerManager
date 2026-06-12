import { onMounted, ref, type Ref } from "vue";
import { ApiError } from "@/lib/api";
import type { presetResource } from "@/lib/presets";
import type { DropDownList } from "@/types/generated";

// Common controller for the four preset pages: list + select + create +
// delete + save, with notice/error handling.
export function usePresetPage<T extends { id?: number }>(
  resource: ReturnType<typeof presetResource<T>>,
  prepare?: (form: T) => T,
) {
  const items: Ref<DropDownList[]> = ref([]);
  const selectedId = ref<number | null>(null);
  const form = ref<T | null>(null) as Ref<T | null>;
  const busy = ref(false);
  const notice = ref("");
  const error = ref("");

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
      if (!window.confirm("Delete this preset? Events using it will block the delete.")) return;
      await resource.remove(id);
      if (selectedId.value === id) {
        selectedId.value = null;
        form.value = null;
      }
      await reloadList();
    });

  const save = () =>
    guard(async () => {
      if (!form.value || selectedId.value === null) return;
      await resource.update(selectedId.value, form.value);
      await reloadList();
      markClean();
      notice.value = "Saved.";
    });

  onMounted(() => guard(reloadList));

  return { items, selectedId, form, busy, notice, error, select, create, remove, save, isDirty };
}
