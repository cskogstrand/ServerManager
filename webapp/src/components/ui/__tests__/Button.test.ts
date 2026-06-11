import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import Button from "../Button.vue";

describe("Button", () => {
  it("renders slot content", () => {
    const wrapper = mount(Button, { slots: { default: "Start Server" } });
    expect(wrapper.text()).toBe("Start Server");
  });

  it("applies the variant class", () => {
    const wrapper = mount(Button, { props: { variant: "danger" } });
    expect(wrapper.classes().join(" ")).toContain("text-danger");
  });

  it("respects disabled", () => {
    const wrapper = mount(Button, { props: { disabled: true } });
    expect(wrapper.attributes("disabled")).toBeDefined();
  });
});
