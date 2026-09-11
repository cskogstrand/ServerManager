<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useContentStore } from "@/stores/content";
import type { CarRef, TrackRef } from "@/types/driverStats";
import TrackImage from "@/components/TrackImage.vue";
import Icon from "@/components/ui/Icon.vue";

const props = defineProps<{ car?: CarRef | null; track?: TrackRef | null }>();
const content = useContentStore();
const carImage = computed(() => {
  if (!props.car) return "";
  const car = content.carByKey(props.car.key);
  const skin = car?.skins.find(s => s.key === props.car?.skin)?.key ?? car?.skins[0]?.key ?? props.car.skin;
  return skin ? `/api/car/image/${encodeURIComponent(props.car.key)}/${encodeURIComponent(skin)}?v=${content.imageVersion}` : "";
});
const imageOk = ref(true);
watch(carImage, () => { imageOk.value = true; });
const trackVersion = computed(() => props.track ? content.trackByKey(props.track.key, props.track.config ?? "")?.version : "");
</script>

<template>
  <section class="profile-favourites" aria-label="Driver favourites">
    <article>
      <div v-if="car" class="profile-favourite-art profile-favourite-car">
        <img v-if="carImage && imageOk" :src="carImage" :alt="car.name" @error="imageOk = false" />
        <Icon v-else name="car" :size="48" />
      </div>
      <div class="profile-favourite-caption">
        <h2>Favourite car</h2>
        <h3>{{ car?.name || 'Still finding a favourite.' }}</h3>
        <p>{{ car ? (car.skin ? `Livery · ${car.skin}` : 'The car they drive most.') : 'Their most-driven car will appear here.' }}</p>
      </div>
    </article>
    <article>
      <TrackImage v-if="track" :track-key="track.key" :config="track.config" class="profile-favourite-art" />
      <div class="profile-favourite-caption">
        <h2>Favourite track</h2>
        <h3>{{ track?.name || 'The next favourite is out there.' }}</h3>
        <p>{{ track ? [track.country, trackVersion ? `Version ${trackVersion}` : '', 'Their most-driven layout.'].filter(Boolean).join(' · ') : 'Their most-driven track will appear here.' }}</p>
      </div>
    </article>
  </section>
</template>
