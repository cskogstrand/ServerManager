import { defineStore } from "pinia";
import { api } from "@/lib/api";
import type { CacheCar, CacheTrack, CacheWeather } from "@/types/generated";
import type { ServerEvent } from "@/lib/sse";

export interface ContentJob {
  id: string;
  kind: string;
  source: string;
  source_name: string;
  status: string;
  phase: string;
  message: string;
  progress: number;
  tracks_total: number;
  cars_total: number;
  weathers_total: number;
  started_at: number;
  updated_at: number;
  finished_at: number;
}

export const useContentStore = defineStore("content", {
  state: () => ({
    cars: [] as CacheCar[],
    tracks: [] as CacheTrack[],
    weathers: [] as CacheWeather[],
    loaded: false,
    jobs: {} as Record<string, ContentJob>,
  }),

  getters: {
    jobList(state): ContentJob[] {
      return Object.values(state.jobs).sort((a, b) => b.updated_at - a.updated_at);
    },
    activeJobs(): ContentJob[] {
      return this.jobList.filter((j) => j.status === "queued" || j.status === "running");
    },
  },

  actions: {
    async load(force = false) {
      if (this.loaded && !force) return;
      const [cars, tracks, weathers] = await Promise.all([
        api.get<{ items: CacheCar[] }>("/api/cars"),
        api.get<{ items: CacheTrack[] }>("/api/tracks"),
        api.get<{ items: CacheWeather[] }>("/api/weathers"),
      ]);
      this.cars = cars.items;
      this.tracks = tracks.items;
      this.weathers = weathers.items;
      this.loaded = true;
    },

    // Fed from the SSE stream (content_job events) by the server store
    applyJobEvent(event: ServerEvent) {
      const job = event.data as ContentJob;
      if (!job?.id) return;
      this.jobs[job.id] = job;
      // A finished import means new content — refresh the caches
      if (job.status === "completed") void this.load(true);
    },

    carByKey(key: string | undefined | null): CacheCar | undefined {
      return key ? this.cars.find((c) => c.key === key) : undefined;
    },
  },
});
