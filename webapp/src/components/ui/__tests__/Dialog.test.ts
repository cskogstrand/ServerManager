import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import Modal from "../Modal.vue";
import { activeDialog } from "@/lib/useDialog";
const wrappers: VueWrapper[] = [];
beforeEach(() => {
  // jsdom lacks native dialog operations; actual keyboard containment is checked in the browser.
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", { configurable: true, value: vi.fn(function (this: HTMLDialogElement) { this.open = true; }) });
  Object.defineProperty(HTMLDialogElement.prototype, "close", { configurable: true, value: vi.fn(function (this: HTMLDialogElement) { this.open = false; }) });
});
afterEach(() => { wrappers.reverse().splice(0).forEach(wrapper => wrapper.unmount()); Reflect.deleteProperty(HTMLDialogElement.prototype, "showModal"); Reflect.deleteProperty(HTMLDialogElement.prototype, "close"); document.body.innerHTML = ""; });
it("keeps cancel controlled, tracks nested dialogs and restores the trigger", async () => {
  const trigger = document.createElement("button"); document.body.append(trigger); trigger.focus();
  const editor = mount(Modal, { props: { open: true, title: "Edit race" }, attachTo: document.body }); wrappers.push(editor);
  await editor.vm.$nextTick();
  const outer = activeDialog.value!;
  expect(outer.open).toBe(true);
  const event = new Event("cancel", { cancelable: true }); outer.dispatchEvent(event);
  expect(event.defaultPrevented).toBe(true); expect(editor.emitted("close")).toHaveLength(1); expect(outer.open).toBe(true);
  const confirm = mount(Modal, { props: { open: true, title: "Discard changes?" }, attachTo: document.body }); wrappers.push(confirm);
  await confirm.vm.$nextTick(); expect(activeDialog.value).not.toBe(outer);
  await confirm.setProps({ open: false }); expect(activeDialog.value).toBe(outer);
  await editor.setProps({ open: false }); expect(activeDialog.value).toBeNull(); expect(document.activeElement).toBe(trigger);
});
