import { api } from "@/lib/api";
import type { DropDownList } from "@/types/generated";

// All four preset types share the same REST shape:
//   GET  /api/<plural>          → {items}
//   POST /api/<plural> {name}   → {id}
//   GET  /api/<singular>/:id    → {data}
//   PUT  /api/<singular>/:id    → {id}
//   DELETE /api/<singular>/:id  → {id}
export function presetResource<T>(plural: string, singular: string) {
  return {
    list: async () => (await api.get<{ items: DropDownList[] }>(`/api/${plural}`)).items,
    create: async (name: string) => (await api.post<{ id: number }>(`/api/${plural}`, { name })).id,
    get: async (id: number) => (await api.get<{ data: T }>(`/api/${singular}/${id}`)).data,
    update: (id: number, body: T) => api.put(`/api/${singular}/${id}`, body),
    remove: (id: number) => api.delete(`/api/${singular}/${id}`),
  };
}
