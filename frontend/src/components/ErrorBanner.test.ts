// @vitest-environment jsdom

import { expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0));

it("renders message and hides details by default", async () => {
  const { default: ErrorBanner } = await import("@/components/ErrorBanner.vue");

  const wrapper = mount(ErrorBanner, {
    props: {
      error: {
        message: "Boom",
        code: "bad_request",
        requestId: "req_123",
        hint: "Try again",
      },
    },
  });

  expect(wrapper.get('[data-testid="error-banner-message"]').text()).toContain("Boom");
  expect(wrapper.find('[data-testid="error-banner-details"]').exists()).toBe(false);

  await wrapper.get('[data-testid="error-banner-toggle"]').trigger("click");
  await flushPromises();

  expect(wrapper.find('[data-testid="error-banner-details"]').exists()).toBe(true);
  expect(wrapper.get('[data-testid="error-banner-request-id"]').text()).toContain("req_123");

  wrapper.unmount();
});

it("copies request_id when requested", async () => {
  const { default: ErrorBanner } = await import("@/components/ErrorBanner.vue");

  const writeText = vi.fn(async () => undefined);
  vi.stubGlobal("navigator", {
    clipboard: { writeText },
  } as any);

  const wrapper = mount(ErrorBanner, {
    props: {
      error: {
        message: "Boom",
        requestId: "req_456",
      },
    },
  });

  await wrapper.get('[data-testid="error-banner-toggle"]').trigger("click");
  await flushPromises();
  await wrapper.get('[data-testid="error-banner-copy-request-id"]').trigger("click");
  await flushPromises();

  expect(writeText).toHaveBeenCalledWith("req_456");

  wrapper.unmount();
});

