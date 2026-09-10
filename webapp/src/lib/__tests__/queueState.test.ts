import { describe, expect, it } from "vitest";
import { queueState } from "../queueState";
describe("run-plan state", () => {
  it("never calls a stopped server's unfinished event active", () => {
    expect(queueState({ finished: 0, started_at: 100 }, false)).toBe("pending");
    expect(queueState({ finished: 0, started_at: 100 }, true)).toBe("active");
  });
  it("requires a start timestamp and gives completed rows precedence", () => {
    expect(queueState({ finished: 0, started_at: null }, true)).toBe("pending");
    expect(queueState({ finished: 1, started_at: 100 }, true)).toBe("done");
    expect(queueState({ finished: true }, false)).toBe("done");
  });
});
