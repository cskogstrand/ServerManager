<script setup lang="ts">
// Single global confirm dialog driven by the confirm store. Mounted once in
// App.vue; any code can call useConfirmStore().ask(...).
import { useConfirmStore } from "@/stores/confirm";
import Modal from "@/components/ui/Modal.vue";
import Button from "@/components/ui/Button.vue";

const confirm = useConfirmStore();
</script>

<template>
  <Modal :open="confirm.open" :title="confirm.title" @close="confirm.answer(false)">
    <p class="text-sm text-text">{{ confirm.message }}</p>
    <p v-if="confirm.detail" class="mt-2 text-sm text-muted">{{ confirm.detail }}</p>
    <template #footer>
      <Button variant="ghost" @click="confirm.answer(false)">{{ confirm.cancelLabel }}</Button>
      <Button :variant="confirm.tone === 'danger' ? 'danger' : 'primary'" @click="confirm.answer(true)">
        {{ confirm.confirmLabel }}
      </Button>
    </template>
  </Modal>
</template>
