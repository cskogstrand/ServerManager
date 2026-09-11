<script setup lang="ts">
import { computed,onMounted,onBeforeUnmount,ref,watch } from 'vue';
import {useVideoSlot} from '@/lib/useVideoSlot';
import WhepPlayer from '@/components/WhepPlayer.vue';
import { isWhepUrl } from '@/lib/useDriverStreams';
const props=defineProps<{url:string;name:string;selected?:boolean}>();
const emit=defineEmits<{playing:[];error:[message:string]}>();
const root=ref<HTMLElement|null>(null),visible=ref(false);let observer:IntersectionObserver|undefined;
const slot=useVideoSlot(computed(()=>visible.value&&!props.selected&&isWhepUrl(props.url)));
onMounted(()=>{observer=new IntersectionObserver(entries=>{visible.value=entries[0]?.isIntersecting??false});if(root.value)observer.observe(root.value)});
onBeforeUnmount(()=>observer?.disconnect());
watch(()=>props.url,()=>emit('error','Playback has not been checked'));
</script>
<template><div ref="root" class="source-player"><template v-if="selected || slot"><WhepPlayer v-if="isWhepUrl(url)" :url="url" :minimal="!selected" @playing="emit('playing')" @error="emit('error',$event)" /><iframe v-else :src="url" :title="name" sandbox="allow-scripts allow-same-origin allow-forms allow-presentation" allow="autoplay; fullscreen; picture-in-picture" loading="lazy" /></template><p v-else>{{ !isWhepUrl(url)?'Open this embedded player to watch':visible?'Preview paused · four cameras play at a time':'Playback paused off screen' }}</p></div></template>
<style scoped>.source-player{position:relative;min-width:0;min-height:0;width:100%;height:100%;background:#111813;color:#edf1e8;display:grid;place-items:center}.source-player>iframe{position:absolute;inset:0;width:100%;height:100%;border:0}.source-player>p{font-size:12px;color:#adb6aa}</style>
