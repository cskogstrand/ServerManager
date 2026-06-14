import { ref, watch, type Ref } from "vue";
import { useRoute, useRouter } from "vue-router";

export interface QueryParamOptions<T> {
  // Turn the raw URL string into the typed value. Should fall back to a sane
  // value for unexpected input (e.g. a hand-edited ?tab=bogus).
  parse?: (raw: string) => T;
  // Turn the value into a URL string, or null to omit the param.
  serialize?: (v: T) => string | null;
}

// Two-way bind a ref to a URL query param so list selections and filters
// survive refresh and back-navigation.
//
// - Initial value is read from the URL, falling back to `def`.
// - Changes are written with router.replace (overwrite, not push) so the URL
//   reflects the latest selection without stacking history entries.
// - A value equal to `def` drops the param to keep URLs clean.
// - Unrelated query params are preserved.
//
// Each call owns one param key. Pages with several filters call it once per
// key; since a user changes one filter at a time, the per-key writes don't
// clobber each other (each merges the current live query before replacing).
export function useQueryParam<T extends string | number | null>(
  key: string,
  def: T,
  options: QueryParamOptions<T> = {},
): Ref<T> {
  const route = useRoute();
  const router = useRouter();
  const parse = options.parse ?? ((raw: string) => raw as unknown as T);
  const serialize = options.serialize ?? ((v: T) => (v == null || v === "" ? null : String(v)));

  const raw = route.query[key];
  const state = ref(typeof raw === "string" ? parse(raw) : def) as Ref<T>;

  const defStr = serialize(def);
  watch(state, (v) => {
    const next = { ...router.currentRoute.value.query };
    const s = serialize(v);
    if (s == null || s === defStr) delete next[key];
    else next[key] = s;
    void router.replace({ query: next });
  });

  return state;
}

// Helpers for the two common non-string param shapes.

// A nullable numeric id param (e.g. a selected instance or preset).
export function numberParam(): QueryParamOptions<number | null> {
  return {
    parse: (raw) => {
      const n = Number(raw);
      return raw !== "" && Number.isFinite(n) ? n : null;
    },
    serialize: (v) => (v == null ? null : String(v)),
  };
}

// A param constrained to a fixed set of string values, with a fallback.
export function enumParam<T extends string>(allowed: readonly T[], fallback: T): QueryParamOptions<T> {
  return {
    parse: (raw) => (allowed.includes(raw as T) ? (raw as T) : fallback),
    serialize: (v) => String(v),
  };
}
