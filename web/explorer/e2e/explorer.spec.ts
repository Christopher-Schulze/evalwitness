import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const explorerPath = process.env.EVALWITNESS_EXPLORER_HTML;
if (explorerPath === undefined || explorerPath === "") {
  throw new Error("EVALWITNESS_EXPLORER_HTML must point to a rendered evidence explorer.");
}
const explorerURL = pathToFileURL(resolve(explorerPath)).href;

test("keeps the complete evidence journey offline, accessible, and responsive", async ({
  page,
}) => {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const externalRequests: string[] = [];

  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("request", (request) => {
    if (/^(https?|wss?):/u.test(request.url())) {
      externalRequests.push(request.url());
    }
  });

  await page.goto(explorerURL, { waitUntil: "load" });

  expect(consoleErrors).toEqual([]);
  expect(pageErrors).toEqual([]);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByText("Accepted, narrowly scoped", { exact: true })).toBeVisible();
  const breakClaim = page.getByRole("link", { name: "Break this claim" });
  await expect(breakClaim).toHaveAttribute("href", "#challenge");
  await breakClaim.click();
  expect(new URL(page.url()).hash).toBe("#challenge");
  await expect(page.getByRole("button", { name: "Inspect method generation v3" })).toContainText(
    /939.*3.*40.*0/u,
  );
  await expect(page.getByRole("tablist", { name: "Registered challenge classes" })).toBeVisible();

  const relianceLink = page.getByRole("link", { name: "Open evidence reliance map" });
  await relianceLink.click();
  expect(new URL(page.url()).hash).toBe("#reliance");
  const reliance = page.locator("#reliance");
  await expect(
    reliance.getByRole("heading", { name: "What actually moves the verifier?" }),
  ).toBeVisible();
  await expect(
    reliance.getByText("Registered cells", { exact: true }).locator("..").getByText("1536", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    reliance.getByText("Outcome bearing", { exact: true }).locator("..").getByText("1536", {
      exact: true,
    }),
  ).toBeVisible();
  await reliance.getByRole("button", { name: "Decision flip" }).click();
  await reliance.getByRole("button", { name: "Interaction" }).click();
  await reliance
    .getByRole("button", {
      name: "Inspect error_output by prompt_injection decision_flip",
    })
    .click();
  await expect(
    reliance.getByRole("heading", { name: "Error output × Prompt injection" }),
  ).toBeVisible();
  await expect(
    reliance.getByText("Selector alignment is not defined for this interaction."),
  ).toBeVisible();
  await expect(reliance.getByText("raw trajectory content shown=false")).toBeVisible();
  await reliance.getByRole("button", { name: "Inspect map" }).click();
  await expect(page.getByRole("dialog")).toContainText("Evidence reliance map");
  await page.keyboard.press("Escape");

  const identicalResponseLink = page.getByRole("link", { name: "Open exact-response study" });
  if ((await identicalResponseLink.count()) === 1) {
    await identicalResponseLink.click();
    expect(new URL(page.url()).hash).toBe("#identical-response");
    const identicalResponse = page.locator("#identical-response");
    await expect(
      identicalResponse.getByRole("heading", { name: "Same response. Different decision." }),
    ).toBeVisible();
    await expect(identicalResponse.getByText("Split decisions", { exact: true })).toBeVisible();
    await expect(identicalResponse.getByText("7", { exact: true })).toBeVisible();
    await identicalResponse.getByRole("tab", { name: "7 disagreements" }).click();
    await expect(identicalResponse.getByText("gpt2 codegolf", { exact: true })).toBeVisible();
    await identicalResponse.getByRole("tab", { name: "34 receipts" }).click();
    await expect(
      identicalResponse.getByText("digest substitution", { exact: true }).first(),
    ).toBeVisible();
    await expect(
      identicalResponse.getByText("scripts/evals/reproduce-identical-response-v5.sh"),
    ).toBeVisible();
    await identicalResponse.getByRole("button", { name: "Inspect capsule" }).click();
    await expect(page.getByRole("dialog")).toContainText("TASK 070 outer capsule");
    await page.keyboard.press("Escape");
  }

  const stressLink = page.getByRole("link", { name: "Open stress lab" });
  await stressLink.click();
  expect(new URL(page.url()).hash).toBe("#stress");
  const stress = page.locator("#stress");
  await expect(
    stress.getByRole("heading", { name: "The violation survives in two lines." }),
  ).toBeVisible();
  await expect(stress.getByText("53 attempts · 30 removed · 93.75% reduction")).toBeVisible();
  await expect(stress.locator('[data-stress-candidate="trajectory-b"]')).toHaveAttribute(
    "data-selected",
    "true",
  );
  await stress.getByRole("button", { name: "Original" }).click();
  await expect(stress.locator('[data-stress-candidate="trajectory-a"]')).toHaveAttribute(
    "data-selected",
    "true",
  );
  await stress.getByRole("button", { name: /^Inspect reduction step 1:/u }).click();
  await expect(stress.getByText("Step 1 / 53", { exact: true })).toBeVisible();
  await stress.getByRole("button", { name: "Inspect bound artifact" }).click();
  await expect(page.getByRole("dialog")).toContainText("Stress development case study");
  await expect(page.getByRole("dialog")).toContainText("Payload SHA-256");
  await page.keyboard.press("Escape");

  await page.getByRole("tab", { name: "Caveat removal" }).click();
  await expect(page.getByText("Sealed source unchanged", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Inspect receipt" }).click();
  const receiptDialog = page.getByRole("dialog");
  await expect(receiptDialog).toBeVisible();
  await expect(receiptDialog).toContainText("Receipt digest");
  await expect(receiptDialog).toContainText("Replay");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();

  await page.getByRole("button", { name: /Tables|Table fallback/u }).click();
  await expect(page.getByRole("heading", { name: "Method integrity table" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Claim transport table" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Challenge receipt table" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Stress reduction table" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Evidence reliance table" })).toBeVisible();

  await page.getByRole("tab", { name: "Owner inspection" }).click();
  await expect(
    page.getByRole("heading", { name: "Complete inspection, revision still required." }),
  ).toBeVisible();
  await expect(page.getByText("66 / 66", { exact: true })).toBeVisible();
  await expect(
    page.getByText("This is a verified public aggregate over a withheld private chain."),
  ).toBeVisible();

  await page.goto(`${explorerURL}#autopsy/method/v3`, { waitUntil: "load" });
  await expect(page.getByRole("dialog")).toContainText("Method generation v3");
  await expect(page.getByRole("dialog")).toContainText("scarcity admitted");
  await page.keyboard.press("Escape");

  await page.goto(`${explorerURL}#challenge/CLM-011/clm-011.denominator-deletion`, {
    waitUntil: "load",
  });
  await expect(page.getByRole("dialog")).toContainText("denominator deletion");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("tab", { name: /^denominator deletion$/iu })).toHaveAttribute(
    "data-state",
    "active",
  );

  await page.goto(`${explorerURL}#unregistered`, { waitUntil: "load" });
  await expect.poll(() => new URL(page.url()).hash).toBe("#autopsy");
  await expect(page.getByRole("dialog")).toBeHidden();

  const layout = await page.evaluate(() => {
    function hasClippingAncestor(element: HTMLElement): boolean {
      let parent = element.parentElement;
      while (parent !== null && parent !== document.body) {
        const overflow = getComputedStyle(parent).overflowX;
        if (["auto", "clip", "hidden", "scroll"].includes(overflow)) {
          return true;
        }
        parent = parent.parentElement;
      }
      return false;
    }

    return {
      bodyFits: document.body.scrollWidth <= document.body.clientWidth,
      bodyWidth: `${document.body.scrollWidth}/${document.body.clientWidth}`,
      documentFits: document.documentElement.scrollWidth <= document.documentElement.clientWidth,
      documentWidth: `${document.documentElement.scrollWidth}/${document.documentElement.clientWidth}`,
      overflowing: Array.from(document.querySelectorAll<HTMLElement>("body *"))
        .filter((element) => {
          const bounds = element.getBoundingClientRect();
          return (
            !hasClippingAncestor(element) &&
            (bounds.left < -0.5 || bounds.right > document.documentElement.clientWidth + 0.5)
          );
        })
        .slice(0, 20)
        .map((element) => ({
          className: element.className,
          right: Math.round(element.getBoundingClientRect().right),
          tag: element.tagName.toLowerCase(),
          text: element.textContent?.trim().slice(0, 80) ?? "",
        })),
      reducedMotion: window.matchMedia("(prefers-reduced-motion: reduce)").matches,
    };
  });
  expect(layout).toMatchObject({
    bodyFits: true,
    documentFits: true,
    overflowing: [],
    reducedMotion: true,
  });
  expect(externalRequests).toEqual([]);
  expect(consoleErrors).toEqual([]);
  expect(pageErrors).toEqual([]);

  // Axe injects a temporary inline style; the report CSP intentionally rejects that style after analysis.
  const accessibility = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();
  const violationSummary = accessibility.violations.map((violation) => ({
    id: violation.id,
    targets: violation.nodes.map((node) => node.target.join(" ")),
  }));
  expect(violationSummary).toEqual([]);
  expect(externalRequests).toEqual([]);
});
