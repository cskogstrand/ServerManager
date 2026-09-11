<script setup lang="ts">
import {computed} from 'vue';
import {screenSetups} from '@/lib/screenSetups';
const props=defineProps<{setup:string}>();
const art=computed(()=>screenSetups.find(s=>s.id===props.setup)??screenSetups[0]);
</script>
<template><img :src="art.image" alt="" class="screen-setup-art"/></template>
<style scoped>.screen-setup-art{width:100%;height:100%;object-fit:contain}</style>
