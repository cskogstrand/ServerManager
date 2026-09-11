<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute } from 'vue-router';
import { advancedSections, advancedTools } from '@/lib/advancedCatalogue';
import { useAuthStore } from '@/stores/auth';
import { useServerStore } from '@/stores/server';
const route=useRoute(), auth=useAuthStore(), server=useServerStore();
const search=ref('');
const visible=computed(()=>advancedTools.filter(t=>(t.role==='viewer'||auth.isAdmin||(t.role==='steward'&&auth.canOperate))&&(!route.params.section||t.section===route.params.section)&&[t.title,t.description,...t.labels].join(' ').toLowerCase().includes(search.value.toLowerCase().trim())));
function destination(path:string) { return path==='/server' ? (server.selectedInstanceId ? `/server/${server.selectedInstanceId}` : '/garage') : path==='/queue' ? `${path}?instance=${server.selectedInstanceId || ''}` : path; }
</script>
<template>
<header class="pitlane-heading"><div><span class="pitlane-eyebrow">Garage / Advanced</span><h1>Find the right control<span class="text-accent">.</span></h1><p>Search by what you need to do, or the setting you already know.</p></div><RouterLink to="/garage" class="pitlane-button">Back to Garage</RouterLink></header>
<div class="advanced-toolbar"><label class="pitlane-field"><span>Search tools and settings</span><input v-model="search" type="search" placeholder="Ports, multiplier, missing stream…" /></label><label class="pitlane-field"><span>Target for server operations</span><select :value="server.selectedInstanceId" @change="server.selectInstance(Number(($event.target as HTMLSelectElement).value))"><option v-if="!server.instanceList.length" value="">No servers available</option><option v-for="i in server.instanceList" :key="i.id" :value="i.id">{{ i.name }}</option></select></label></div>
<p class="text-sm text-muted mb-7">Installation, account, content and global settings keep their own scope. Selecting a server affects only server operations.</p>
<div class="advanced-layout"><nav aria-label="Advanced sections" class="advanced-sections"><RouterLink to="/garage/advanced" :aria-current="!route.params.section ? 'page' : undefined">All tools</RouterLink><RouterLink v-for="section in advancedSections" :key="section.id" :to="`/garage/advanced/${section.id}`" :aria-current="route.params.section===section.id ? 'page' : undefined">{{ section.name }}</RouterLink></nav><div>
<p v-if="!visible.length" class="pitlane-empty">No matching tools available for your role.</p>
<section v-for="section in advancedSections.filter(s=>visible.some(t=>t.section===s.id))" :key="section.id" class="mb-8"><span class="pitlane-eyebrow">{{ section.name }}</span><div class="pitlane-stack"><RouterLink v-for="tool in visible.filter(t=>t.section===section.id)" :key="tool.id" :to="destination(tool.path)" class="pitlane-card advanced-tool"><h2>{{ tool.title }} <span aria-hidden="true">↗</span></h2><p>{{ tool.description }}</p><small class="text-muted block mt-3">{{ tool.scope }}</small></RouterLink></div></section>
</div></div>
</template>
<style scoped>
.advanced-toolbar{display:grid;grid-template-columns:2fr 1fr;gap:24px;margin-bottom:12px}.advanced-layout{display:grid;grid-template-columns:230px minmax(0,1fr);gap:36px}.advanced-sections{display:flex;flex-direction:column;align-self:start;position:sticky;top:20px}.advanced-sections a{min-height:44px;padding:11px 14px;border-radius:8px;font-size:14px;color:var(--color-muted)}.advanced-sections a[aria-current=page]{background:var(--color-surface-2);color:var(--color-text)}.advanced-tool h2{font-size:18px;display:flex;justify-content:space-between;gap:15px}.advanced-tool:hover{border-color:var(--color-accent)}@media(max-width:767px){.advanced-layout,.advanced-toolbar{grid-template-columns:1fr}.advanced-sections{position:static;flex-direction:row;overflow:auto}.advanced-sections a{white-space:nowrap}}
</style>
