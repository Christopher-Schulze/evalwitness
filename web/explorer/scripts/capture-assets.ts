import { createHash } from "node:crypto";
import { mkdir, readFile, stat, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

import { chromium, type Browser, type Page, type ViewportSize } from "@playwright/test";

import { reportSchema, type EvidenceReport } from "../src/lib/report";

interface CaptureOptions {
  html: string;
  destination: string;
}

interface RenderBindings {
  report: EvidenceReport;
  reportDigest: string;
  rendererDigest: string;
}

interface CapturedAsset {
  path: string;
  bytes: Buffer;
}

const desktopViewport = { width: 1440, height: 1000 } satisfies ViewportSize;
const mobileViewport = { width: 390, height: 844 } satisfies ViewportSize;

async function main(): Promise<void> {
  const options = parseArguments(process.argv.slice(2));
  const html = resolve(options.html);
  const destination = resolve(options.destination);
  const input = await stat(html);
  if (!input.isFile()) {
    throw new Error("--html must identify one regular rendered explorer file");
  }
  await assertDestinationAbsent(destination);

  const executablePath = process.env.EVALWITNESS_BROWSER_EXECUTABLE;
  const browser = await chromium.launch({
    headless: true,
    ...(executablePath === undefined ? {} : { executablePath }),
  });
  try {
    const desktop = await capture(browser, html, desktopViewport, "autopsy");
    const mobile = await capture(browser, html, mobileViewport, "autopsy");
    const stress = await capture(browser, html, desktopViewport, "stress");
    const reliance = await capture(browser, html, desktopViewport, "reliance");
    const identicalResponse = await capture(
      browser,
      html,
      desktopViewport,
      "identical-response",
      true,
    );
    const ownerInspection = await capture(browser, html, desktopViewport, "owner-inspection");
    const bindings = await readBindings(browser, html);
    const architecture = Buffer.from(renderArchitecture(bindings), "utf8");
    const assets: CapturedAsset[] = [
      { path: "claim-autopsy-desktop.png", bytes: desktop },
      { path: "claim-autopsy-mobile.png", bytes: mobile },
      { path: "stress-lab-desktop.png", bytes: stress },
      { path: "evidence-reliance-desktop.png", bytes: reliance },
      ...(identicalResponse === undefined
        ? []
        : [{ path: "identical-response-desktop.png", bytes: identicalResponse }]),
      { path: "owner-inspection-desktop.png", bytes: ownerInspection },
      { path: "architecture.svg", bytes: architecture },
    ];
    const sourceBytes = await readFile(html);
    const manifest = renderManifest(sourceBytes, bindings, assets);

    await mkdir(destination, { mode: 0o755, recursive: true });
    for (const asset of assets) {
      await writeFile(join(destination, asset.path), asset.bytes, { flag: "wx", mode: 0o644 });
    }
    await writeFile(join(destination, "manifest.json"), manifest, { flag: "wx", mode: 0o644 });
  } finally {
    await browser.close();
  }
  console.log(`captured explorer assets in ${destination}`);
}

function parseArguments(args: string[]): CaptureOptions {
  let html = "";
  let destination = "";
  for (let index = 0; index < args.length; index += 2) {
    const name = args[index];
    const value = args[index + 1];
    if (value === undefined || (name !== "--html" && name !== "--destination")) {
      throw new Error(
        "usage: bun scripts/capture-assets.ts --html PATH --destination NEW_DIRECTORY",
      );
    }
    if (name === "--html") {
      html = value;
    } else {
      destination = value;
    }
  }
  if (html === "" || destination === "") {
    throw new Error("--html and --destination are required");
  }
  return { html, destination };
}

async function assertDestinationAbsent(destination: string): Promise<void> {
  try {
    await stat(destination);
  } catch (error) {
    if (isNodeError(error) && error.code === "ENOENT") {
      return;
    }
    throw error;
  }
  throw new Error(`destination already exists: ${destination}`);
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}

async function capture(
  browser: Browser,
  html: string,
  viewport: ViewportSize,
  view: "autopsy" | "identical-response" | "owner-inspection" | "reliance" | "stress",
  optional: true,
): Promise<Buffer | undefined>;
async function capture(
  browser: Browser,
  html: string,
  viewport: ViewportSize,
  view: "autopsy" | "identical-response" | "owner-inspection" | "reliance" | "stress",
  optional?: false,
): Promise<Buffer>;
async function capture(
  browser: Browser,
  html: string,
  viewport: ViewportSize,
  view: "autopsy" | "identical-response" | "owner-inspection" | "reliance" | "stress",
  optional = false,
): Promise<Buffer | undefined> {
  const context = await browser.newContext({
    colorScheme: "light",
    deviceScaleFactor: 1,
    reducedMotion: "reduce",
    serviceWorkers: "block",
    viewport,
  });
  try {
    const page = await context.newPage();
    const errors = monitorRuntime(page);
    await page.goto(pathToFileURL(html).href, { waitUntil: "load" });
    await page.getByRole("heading", { level: 1 }).waitFor({ state: "visible" });
    await page.evaluate(async () => document.fonts.ready);
    if (view === "identical-response") {
      const panel = page.locator("#identical-response");
      if ((await panel.count()) === 0 && optional) {
        return undefined;
      }
      await panel.waitFor({ state: "visible" });
      await panel.scrollIntoViewIfNeeded();
    } else if (view === "owner-inspection") {
      await page.getByRole("tab", { name: "Owner inspection" }).click();
      const panel = page.locator("#owner-inspection");
      await panel.waitFor({ state: "visible" });
      await panel.scrollIntoViewIfNeeded();
    } else if (view === "reliance") {
      const panel = page.locator("#reliance");
      await panel.waitFor({ state: "visible" });
      await panel.scrollIntoViewIfNeeded();
    } else if (view === "stress") {
      const panel = page.locator("#stress");
      await panel.waitFor({ state: "visible" });
      await panel.scrollIntoViewIfNeeded();
    }
    assertCleanRuntime(errors);
    const first = await page.screenshot({ animations: "disabled", caret: "hide", scale: "css" });
    const second = await page.screenshot({ animations: "disabled", caret: "hide", scale: "css" });
    if (!first.equals(second)) {
      throw new Error(`${view} screenshot is not byte-deterministic within one browser context`);
    }
    assertCleanRuntime(errors);
    return first;
  } finally {
    await context.close();
  }
}

function monitorRuntime(page: Page): { console: string[]; page: string[]; network: string[] } {
  const errors = { console: [] as string[], page: [] as string[], network: [] as string[] };
  page.on("console", (message) => {
    if (message.type() === "error") {
      errors.console.push(message.text());
    }
  });
  page.on("pageerror", (error) => errors.page.push(error.message));
  page.on("request", (request) => {
    if (/^(https?|wss?):/u.test(request.url())) {
      errors.network.push(request.url());
    }
  });
  return errors;
}

function assertCleanRuntime(errors: {
  console: string[];
  page: string[];
  network: string[];
}): void {
  if (errors.console.length !== 0 || errors.page.length !== 0 || errors.network.length !== 0) {
    throw new Error(`asset capture observed runtime activity: ${JSON.stringify(errors)}`);
  }
}

async function readBindings(browser: Browser, html: string): Promise<RenderBindings> {
  const context = await browser.newContext({ serviceWorkers: "block", viewport: desktopViewport });
  try {
    const page = await context.newPage();
    await page.goto(pathToFileURL(html).href, { waitUntil: "load" });
    const embedded = await page.evaluate(() => {
      const report = document.querySelector<HTMLMetaElement>(
        'meta[name="evalwitness-report"]',
      )?.content;
      const renderer = document.querySelector<HTMLMetaElement>(
        'meta[name="evalwitness-renderer-sha256"]',
      )?.content;
      if (report === undefined || renderer === undefined || !/^[0-9a-f]{64}$/u.test(renderer)) {
        throw new Error("rendered explorer metadata is incomplete");
      }
      return { encodedReport: report, rendererDigest: renderer };
    });
    const report = reportSchema.parse(
      JSON.parse(Buffer.from(embedded.encodedReport, "base64").toString("utf8")),
    );
    return { report, reportDigest: report.digest, rendererDigest: embedded.rendererDigest };
  } finally {
    await context.close();
  }
}

function renderManifest(
  sourceHTML: Buffer,
  bindings: RenderBindings,
  assets: CapturedAsset[],
): string {
  return `${JSON.stringify(
    {
      schema_version: "evalwitness.evidence-explorer-public-assets.v2",
      source_html_sha256: digest(sourceHTML),
      report_digest: bindings.reportDigest,
      renderer_digest: bindings.rendererDigest,
      reference_browser: "playwright-chromium-1.62.1",
      files: assets.map((asset) => ({
        path: asset.path,
        bytes: asset.bytes.length,
        sha256: digest(asset.bytes),
      })),
    },
    null,
    2,
  )}\n`;
}

function renderArchitecture(bindings: RenderBindings): string {
  const report = bindings.reportDigest.slice(0, 12);
  const renderer = bindings.rendererDigest.slice(0, 12);
  const generations = bindings.report.method.generations
    .map((generation) => generation.generation)
    .join(" → ");
  const currentGeneration = bindings.report.method.generations.find(
    (generation) => generation.generation === bindings.report.method.current,
  );
  if (currentGeneration === undefined) {
    throw new Error("architecture source report lacks its current method generation");
  }
  const denominator = (name: string): number => {
    const count = currentGeneration.frozen_denominators.find((entry) => entry.name === name);
    if (count === undefined) {
      throw new Error(`architecture source report lacks denominator ${name}`);
    }
    return count.value;
  };
  const transportLayers = bindings.report.transport.paths[0]?.layers.length;
  if (transportLayers === undefined || transportLayers === 0) {
    throw new Error("architecture source report lacks transport layers");
  }
  const methodLabel = escapeXML(generations);
  const selectedLayer = escapeXML(humanize(bindings.report.transport.selected_layer));
  const overallStatus = escapeXML(
    humanize(bindings.report.owner_inspection.outcomes.overall_status),
  );
  const humanStudyStatus = escapeXML(humanize(bindings.report.owner_inspection.human_study_status));
  return `<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="900" viewBox="0 0 1600 900" role="img" aria-labelledby="title description">
  <title id="title">EvalWitness offline evidence explorer architecture</title>
  <desc id="description">A verified public capsule, release, and development stress witness are projected into a canonical report, embedded by a deterministic renderer, and inspected offline through method, transport, reduction, challenge, and custody views.</desc>
  <rect width="1600" height="900" fill="#f4f5f2"/>
  <path d="M0 120H1600M0 780H1600" stroke="#dfe2df"/>
  <g font-family="Inter,Arial,sans-serif" fill="#181c23">
    <text x="96" y="78" font-size="22" font-weight="700" letter-spacing="4">EVALWITNESS / OFFLINE CLAIM AUTOPSY</text>
    <text x="1504" y="78" text-anchor="end" font-family="monospace" font-size="16" fill="#596270">REPORT ${report} · RENDERER ${renderer}</text>
    <g transform="translate(96 170)">
      <rect width="310" height="168" rx="22" fill="#fff" stroke="#c6cbd0"/>
      <circle cx="42" cy="42" r="18" fill="#1d5ee8"/>
      <text x="76" y="38" font-size="13" font-weight="700" letter-spacing="2" fill="#596270">VERIFIED INPUT</text>
      <text x="28" y="88" font-size="28" font-weight="700">Public capsule</text>
      <text x="28" y="119" font-size="16" fill="#596270">Ledger · autopsy · ${bindings.report.release.files_verified} release files</text>
      <text x="28" y="146" font-family="monospace" font-size="14" fill="#1249c1">${bindings.report.scope.provider_calls} provider calls</text>
    </g>
    <path d="M424 254H518" stroke="#1d5ee8" stroke-width="4"/>
    <path d="m508 244 14 10-14 10" fill="none" stroke="#1d5ee8" stroke-width="4"/>
    <g transform="translate(538 170)">
      <rect width="310" height="168" rx="22" fill="#fff" stroke="#1d5ee8" stroke-width="2"/>
      <text x="28" y="40" font-size="13" font-weight="700" letter-spacing="2" fill="#1249c1">CANONICAL PROJECTION</text>
      <text x="28" y="88" font-size="28" font-weight="700">Evidence report</text>
      <text x="28" y="119" font-size="16" fill="#596270">Strict schema · typed absence · SHA-256</text>
      <text x="28" y="146" font-family="monospace" font-size="14" fill="#1249c1">${escapeXML(bindings.report.schema_version)}</text>
    </g>
    <path d="M866 254H960" stroke="#1d5ee8" stroke-width="4"/>
    <path d="m950 244 14 10-14 10" fill="none" stroke="#1d5ee8" stroke-width="4"/>
    <g transform="translate(980 170)">
      <rect width="310" height="168" rx="22" fill="#252a32"/>
      <text x="28" y="40" font-size="13" font-weight="700" letter-spacing="2" fill="#aebbd0">EMBEDDED RENDERER</text>
      <text x="28" y="88" font-size="28" font-weight="700" fill="#fff">One HTML file</text>
      <text x="28" y="119" font-size="16" fill="#cbd2dc">CSP · escaped data · bound assets</text>
      <text x="28" y="146" font-family="monospace" font-size="14" fill="#8eb2ff">file:// · no backend</text>
    </g>
    <path d="M1308 254H1416" stroke="#1d5ee8" stroke-width="4"/>
    <path d="m1406 244 14 10-14 10" fill="none" stroke="#1d5ee8" stroke-width="4"/>
    <circle cx="1470" cy="254" r="34" fill="#edf3ff" stroke="#1d5ee8" stroke-width="2"/>
    <text x="1470" y="260" text-anchor="middle" font-size="18" font-weight="700" fill="#1249c1">UI</text>
    <text x="1470" y="312" text-anchor="middle" font-size="14" fill="#596270">offline browser</text>

    <text x="96" y="410" font-size="13" font-weight="700" letter-spacing="3" fill="#596270">FIVE FALSIFIABLE READER PATHS</text>
    <g transform="translate(96 446)">
      <rect width="264" height="242" rx="22" fill="#fff0ec" stroke="#bf321d"/>
      <text x="26" y="39" font-size="13" font-weight="700" letter-spacing="2" fill="#9d2817">01 / METHOD INTEGRITY</text>
      <text x="26" y="88" font-size="24" font-weight="700">${methodLabel}</text>
      <text x="26" y="122" font-size="15" fill="#596270">${bindings.report.method.transitions.length} preserved breaks</text>
      <text x="26" y="151" font-size="15" fill="#596270">${denominator("natural_total_attempts")} attempts · ${denominator("scarcity_admitted")}/${denominator("scarcity_target")} scarcity</text>
      <text x="26" y="180" font-size="15" fill="#9d2817">${denominator("scarcity_test_role")} held-out test cases</text>
      <rect x="26" y="201" width="154" height="26" rx="13" fill="#bf321d"/>
      <text x="103" y="219" text-anchor="middle" font-size="12" font-weight="700" fill="#fff">SELF-FALSIFIED</text>
    </g>
    <g transform="translate(382 446)">
      <rect width="264" height="242" rx="22" fill="#edf3ff" stroke="#1d5ee8"/>
      <text x="26" y="39" font-size="13" font-weight="700" letter-spacing="2" fill="#1249c1">02 / CLAIM TRANSPORT</text>
      <text x="26" y="88" font-size="24" font-weight="700">${transportLayers} layers</text>
      <text x="26" y="122" font-size="15" fill="#596270">Accepted BOM path</text>
      <text x="26" y="151" font-size="15" fill="#596270">Earliest-loss certificate</text>
      <text x="26" y="180" font-size="15" fill="#1249c1">First loss: ${selectedLayer}</text>
      <rect x="26" y="201" width="140" height="26" rx="13" fill="#1d5ee8"/>
      <text x="96" y="219" text-anchor="middle" font-size="12" font-weight="700" fill="#fff">TRACE BOUND</text>
    </g>
    <g transform="translate(668 446)">
      <rect width="264" height="242" rx="22" fill="#252a32"/>
      <text x="26" y="39" font-size="13" font-weight="700" letter-spacing="2" fill="#aebbd0">03 / STRESS WITNESS</text>
      <text x="26" y="88" font-size="24" font-weight="700" fill="#fff">${bindings.report.stress.original_line_units} → ${bindings.report.stress.final_line_units} lines</text>
      <text x="26" y="122" font-size="15" fill="#cbd2dc">${bindings.report.stress.reduction_attempts} replayed attempts</text>
      <text x="26" y="151" font-size="15" fill="#cbd2dc">${bindings.report.stress.accepted_reductions} accepted removals</text>
      <text x="26" y="180" font-size="15" fill="#ff9584">${humanize(bindings.report.stress.outcome)} · ${bindings.report.stress.provider_calls} provider calls</text>
      <rect x="26" y="201" width="154" height="26" rx="13" fill="#bf321d"/>
      <text x="103" y="219" text-anchor="middle" font-size="12" font-weight="700" fill="#fff">ONE-MINIMAL</text>
    </g>
    <g transform="translate(954 446)">
      <rect width="264" height="242" rx="22" fill="#fff" stroke="#c6cbd0"/>
      <text x="26" y="39" font-size="13" font-weight="700" letter-spacing="2" fill="#596270">04 / BREAK THIS CLAIM</text>
      <text x="26" y="88" font-size="24" font-weight="700">${bindings.report.challenge.classes.length} classes</text>
      <text x="26" y="122" font-size="15" fill="#596270">${bindings.report.challenge.total_receipts} verified receipts</text>
      <text x="26" y="151" font-size="15" fill="#596270">Ephemeral mutations only</text>
      <text x="26" y="180" font-size="15" fill="#9d2817">Sealed source unchanged</text>
      <rect x="26" y="201" width="164" height="26" rx="13" fill="#252a32"/>
      <text x="108" y="219" text-anchor="middle" font-size="12" font-weight="700" fill="#fff">REPLAYABLE</text>
    </g>
    <g transform="translate(1240 446)">
      <rect width="264" height="242" rx="22" fill="#fff6dc" stroke="#b56b00"/>
      <text x="26" y="39" font-size="13" font-weight="700" letter-spacing="2" fill="#825006">05 / CUSTODY BOUNDARY</text>
      <text x="26" y="88" font-size="24" font-weight="700">${bindings.report.owner_inspection.assessments.completed} / ${bindings.report.owner_inspection.assessments.required}</text>
      <text x="26" y="122" font-size="15" fill="#596270">${bindings.report.owner_inspection.dimensions.length} dimensions inspected</text>
      <text x="26" y="151" font-size="15" fill="#9d2817">${overallStatus}</text>
      <text x="26" y="180" font-size="15" fill="#825006">Human study: ${humanStudyStatus}</text>
      <rect x="26" y="201" width="180" height="26" rx="13" fill="#b56b00"/>
      <text x="116" y="219" text-anchor="middle" font-size="12" font-weight="700" fill="#fff">PRIVATE WITHHELD</text>
    </g>
    <text x="96" y="829" font-size="16" fill="#596270">Every visible state resolves to a digest, denominator, guarding test, or typed unavailable contract.</text>
    <text x="1504" y="829" text-anchor="end" font-family="monospace" font-size="14" fill="#596270">${report}</text>
  </g>
</svg>\n`;
}

function humanize(value: string): string {
  return value.replaceAll("_", " ").replaceAll("-", " ");
}

function escapeXML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}

function digest(raw: Buffer): string {
  return createHash("sha256").update(raw).digest("hex");
}

await main();
