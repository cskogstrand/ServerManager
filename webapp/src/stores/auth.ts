import { defineStore } from "pinia";
import { api, ApiError } from "@/lib/api";
import type { Users } from "@/types/generated";

export const useAuthStore = defineStore("auth", {
  state: () => ({
    user: null as Users | null,
    checked: false,
  }),

  getters: {
    loggedIn: (state) => state.user !== null,
    role: (state) => state.user?.role ?? "viewer",
    isAdmin: (state) => state.user?.role === "admin",
    // steward or admin may operate running servers and queues
    canOperate: (state) => state.user?.role === "admin" || state.user?.role === "steward",
  },

  actions: {
    // Resolve session state once per app load (cookie may already be valid)
    async ensureChecked() {
      if (this.checked) return;
      try {
        this.user = await api.get<Users>("/api/user");
      } catch {
        this.user = null;
      } finally {
        this.checked = true;
      }
    },

    async login(name: string, password: string) {
      await api.post("/api/login", { name, password });
      this.user = await api.get<Users>("/api/user");
      this.checked = true;
    },

    async logout() {
      try {
        await api.post("/api/logout");
      } catch (e) {
        if (!(e instanceof ApiError)) throw e;
      }
      this.user = null;
    },

    async refresh() {
      this.user = await api.get<Users>("/api/user");
    },
  },
});
