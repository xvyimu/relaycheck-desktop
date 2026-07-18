import { expect, it } from "vitest";

import { AccountKeySummary } from "../AccountKeySummary";
import type { Account } from "@/types";

it("renders API key diagnostics when model testing succeeds", () => {
  const element = AccountKeySummary({
    account: {
      apiKeyStatus: "valid",
      apiKeySampleModels: ["gpt-4o-mini", "qwen-turbo"],
      apiKeyModelCount: 2,
      apiKeyLatencyMs: 48,
      apiKeyTestModel: "gpt-4o-mini",
      apiKeyModelUsable: true,
      apiKeyLastCheckedAt: "2026-07-17T00:00:00Z",
      apiKeyTestMessage: "模型请求成功",
    } as Account,
  }) as { props: { className: string } };

  expect(element.props.className).toContain("key-valid");
  expect(element.props.className).toContain("is-usable");
});

it("renders safe defaults before API key diagnostics run", () => {
  const element = AccountKeySummary({ account: {} as Account }) as { props: { className: string } };

  expect(element.props.className).toContain("key-unchecked");
});
