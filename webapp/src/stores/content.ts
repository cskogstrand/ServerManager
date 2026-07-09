import { defineStore } from "pinia";
import { api } from "@/lib/api";
import type { CacheCar, CacheTrack, CacheWeather } from "@/types/generated";
import type { ServerEvent } from "@/lib/sse";

export interface ContentJob {
  id: string;
  kind: string;
  source: string;
  source_name: string;
  source_url?: string;
  status: string;
  phase: string;
  message: string;
  progress: number;
  downloaded_bytes: number;
  download_total_bytes: number;
  files_written: number;
  imported_assets?: string[];
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
    // Bumped after re-compressing images; appended to image URLs as a cache
    // buster so freshly compressed previews replace the ones the browser cached.
    imageVersion: 0,
    // Number of compressed previews currently stored in the DB image cache.
    cachedImages: 0,
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

    async loadJobs() {
      const r = await api.get<{ jobs: ContentJob[] }>("/api/content/jobs/active");
      for (const job of r.jobs ?? []) this.upsertJob(job);
    },

    upsertJob(job: ContentJob, refreshOnComplete = false) {
      if (!job?.id) return;
      this.jobs[job.id] = job;
      if (refreshOnComplete && job.status === "completed") {
        void this.load(true);
        void this.loadImageStats();
        this.imageVersion++;
      }
    },

    // Fed from the SSE stream (content_job events) by the server store
    applyJobEvent(event: ServerEvent) {
      const job = event.data as ContentJob;
      // A finished import means new content (already auto-compressed on the
      // server) — refresh the caches, the cached-image count, and bust URLs.
      this.upsertJob(job, true);
    },

    bumpImageVersion() {
      this.imageVersion++;
    },

    async loadImageStats() {
      const r = await api.get<{ cached: number }>("/api/content/images/count");
      this.cachedImages = r.cached;
    },

    carByKey(key: string | undefined | null): CacheCar | undefined {
      return key ? this.cars.find((c) => c.key === key) : undefined;
    },
    trackByKey(key: string | undefined | null, config = ""): CacheTrack | undefined {
      if (!key) return undefined;
      return this.tracks.find((t) => t.key === key && (t.config ?? "") === config) ?? this.tracks.find((t) => t.key === key);
    },

    // Delete content from disk + cache, then drop the local copy. The track key
    // covers every layout, so all matching rows leave the list.
    async deleteTrack(key: string) {
      await api.delete(`/api/track/${encodeURIComponent(key)}`);
      this.tracks = this.tracks.filter((t) => t.key !== key);
    },
    async deleteCar(key: string) {
      await api.delete(`/api/car/${encodeURIComponent(key)}`);
      this.cars = this.cars.filter((c) => c.key !== key);
    },
    async deleteWeather(key: string) {
      await api.delete(`/api/weather/${encodeURIComponent(key)}`);
      this.weathers = this.weathers.filter((w) => w.key !== key);
    },
  },
});
