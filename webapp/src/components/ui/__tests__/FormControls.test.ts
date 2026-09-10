import { afterEach, describe, expect, it } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, ref } from "vue";
import FormRow from "../FormRow.vue";
import Input from "../Input.vue";
import Select from "../Select.vue";
import Combobox from "../Combobox.vue";
import Toggle from "../Toggle.vue";
const wrappers: VueWrapper[] = [];
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); });
describe("accessible form fields", () => {
  it("links visible labels, hints and errors to controls with unique ids", () => {
    const wrapper = mount(defineComponent({
      components: { FormRow, Input, Select, Combobox, Toggle },
      setup: () => ({ text: ref(""), choice: ref(null), enabled: ref(false), options: [{ value: 1, label: "Sprint" }] }),
      template: `<div><FormRow label="Race name" hint="Optional" error="Choose another name"><Input v-model="text" /></FormRow><FormRow label="Server"><Select v-model="choice" :options="options" /></FormRow><FormRow label="Sessions"><Combobox v-model="choice" :options="options" /></FormRow><FormRow label="Enabled"><Toggle v-model="enabled" /></FormRow></div>`,
    }), { attachTo: document.body });
    wrappers.push(wrapper);
    const controls = wrapper.findAll("input, select");
    expect(new Set(controls.map(control => control.attributes("id"))).size).toBe(4);
    for (const control of controls) {
      const id = control.attributes("id");
      expect(wrapper.find(`label[for="${id}"]`).exists()).toBe(true);
      expect(document.getElementById(control.attributes("aria-labelledby")!)?.textContent?.trim()).toBeTruthy();
    }
    expect(controls[0].attributes("aria-invalid")).toBe("true");
    const hints = controls[0].attributes("aria-describedby")!.split(" ").map(id => document.getElementById(id)?.textContent);
    expect(hints).toEqual(["Optional", "Choose another name"]);
    expect(wrapper.find("option:checked").text()).toBe("Choose an option…");
    expect(wrapper.find("option:checked").attributes("disabled")).toBeDefined();
  });
  it("uses keyboard selection, exposes the active option and consumes Escape inside the list", async () => {
    const wrapper = mount(Combobox, { props: { modelValue: 1, ariaLabel: "Sessions", options: [{ value: 1, label: "Practice" }, { value: 2, label: "Race" }] }, attachTo: document.body });
    wrappers.push(wrapper);
    const input = wrapper.get("input");
    await input.trigger("focus");
    expect(input.attributes("aria-expanded")).toBe("true");
    await input.trigger("keydown", { key: "ArrowDown" });
    expect(document.getElementById(input.attributes("aria-activedescendant")!)?.textContent).toBe("Race");
    await input.trigger("keydown", { key: "Enter" });
    expect(wrapper.emitted("update:modelValue")?.[0]).toEqual([2]);
    expect(input.attributes("aria-expanded")).toBe("false");
    await input.trigger("keydown", { key: "ArrowDown" });
    const escape = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
    input.element.dispatchEvent(escape);
    await wrapper.vm.$nextTick();
    expect(escape.defaultPrevented).toBe(true);
    expect(input.attributes("aria-expanded")).toBe("false");
    expect(input.attributes("aria-activedescendant")!).toBeUndefined();
  });
});
