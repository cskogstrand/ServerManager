import { presetResource } from "@/lib/presets";
import type { UserClass, UserDifficulty, UserSession, UserTime, UserTimeWeather } from "@/types/generated";

// Shared preset-form plumbing so the full preset pages and the inline
// "create preset" sheet in the event builder behave identically.

export type PresetKind = "class" | "session" | "time" | "difficulty";

export const presetMeta: Record<PresetKind, { title: string; plural: string; singular: string }> = {
  class: { title: "Car class", plural: "classes", singular: "class" },
  session: { title: "Sessions", plural: "sessions", singular: "session" },
  time: { title: "Time & weather", plural: "times", singular: "time" },
  difficulty: { title: "Difficulty", plural: "difficulty", singular: "difficulty" },
};

// presetMeta.difficulty.plural is "difficulty"? The API uses /difficulties.
presetMeta.difficulty.plural = "difficulties";

export function emptyWeather(): UserTimeWeather {
  return {
    graphics: undefined,
    base_temperature_ambient: 20,
    base_temperature_road: 7,
    variation_ambient: 2,
    variation_road: 2,
    wind_base_speed_min: 0,
    wind_base_speed_max: 10,
    wind_base_direction: 0,
    wind_variation_direction: 0,
  } as UserTimeWeather;
}

const numericWeatherFields = [
  "base_temperature_ambient",
  "base_temperature_road",
  "variation_ambient",
  "variation_road",
  "wind_base_speed_min",
  "wind_base_speed_max",
  "wind_base_direction",
  "wind_variation_direction",
  "csp_time_of_day_multi",
] as const;

// Time GET payload serializes weather numbers as strings (legacy ",string"
// tags). Coerce to real numbers and guarantee one weather panel.
export function prepareTime(form: UserTime): UserTime {
  if (!form.weathers || form.weathers.length === 0) {
    form.weathers = [emptyWeather()];
  }
  for (const w of form.weathers) {
    for (const key of numericWeatherFields) {
      const v = (w as any)[key];
      if (typeof v === "string") (w as any)[key] = v === "" ? null : Number(v);
    }
  }
  return form;
}

export function prepareClass(form: UserClass): UserClass {
  form.entries ??= [];
  for (const e of form.entries) e.count ??= 1;
  return form;
}

// Build a typed resource + its prepare hook for one kind.
export function presetFor(kind: PresetKind) {
  switch (kind) {
    case "class":
      return { resource: presetResource<UserClass>("classes", "class"), prepare: prepareClass as (f: any) => any };
    case "session":
      return { resource: presetResource<UserSession>("sessions", "session"), prepare: undefined };
    case "time":
      return { resource: presetResource<UserTime>("times", "time"), prepare: prepareTime as (f: any) => any };
    case "difficulty":
      return { resource: presetResource<UserDifficulty>("difficulties", "difficulty"), prepare: undefined };
  }
}
