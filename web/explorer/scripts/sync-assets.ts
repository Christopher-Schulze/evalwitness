import { createHash } from "node:crypto";
import { mkdir, readFile, rename, stat, writeFile } from "node:fs/promises";
import path from "node:path";

interface AssetRecord {
  path: string;
  bytes: number;
  sha256: string;
}

interface AssetCandidate extends AssetRecord {
  content: Uint8Array;
}

const sourceRoot = path.resolve(import.meta.dirname, "../dist");
const targetRoot = path.resolve(import.meta.dirname, "../../../internal/explorer/assets");

async function main(): Promise<void> {
  const mode = parseMode(process.argv.slice(2));
  const candidates = await buildCandidates();
  if (mode === "--check") {
    await checkCandidates(candidates);
    return;
  }
  if (mode === "--candidate") {
    await writeCandidateFiles(candidates);
    return;
  }
  await writeCandidates(candidates);
}

function parseMode(args: string[]): "--candidate" | "--check" | "--write" {
  if (
    args.length !== 1 ||
    (args[0] !== "--candidate" && args[0] !== "--check" && args[0] !== "--write")
  ) {
    throw new Error("usage: bun scripts/sync-assets.ts --candidate|--check|--write");
  }
  return args[0];
}

async function buildCandidates(): Promise<AssetCandidate[]> {
  const javascript = await readRequiredAsset("explorer.js");
  const stylesheet = await readRequiredAsset("explorer.css");
  rejectSourceMap(javascript, "explorer.js");
  const records = [assetRecord("explorer.css", stylesheet), assetRecord("explorer.js", javascript)];
  const manifest = new TextEncoder().encode(
    `${JSON.stringify({ files: records, schema_version: "evalwitness.evidence-explorer-assets.v1" })}\n`,
  );
  return [
    { ...records[0]!, content: stylesheet },
    { ...records[1]!, content: javascript },
    { ...assetRecord("manifest.json", manifest), content: manifest },
  ];
}

async function readRequiredAsset(fileName: string): Promise<Uint8Array> {
  const content = await readFile(path.join(sourceRoot, fileName));
  if (content.byteLength === 0) {
    throw new Error(`built explorer asset ${fileName} is empty`);
  }
  return content;
}

function rejectSourceMap(content: Uint8Array, fileName: string): void {
  if (new TextDecoder().decode(content).includes("sourceMappingURL=")) {
    throw new Error(`built explorer asset ${fileName} contains a source-map reference`);
  }
}

function assetRecord(filePath: string, content: Uint8Array): AssetRecord {
  return { bytes: content.byteLength, path: filePath, sha256: sha256(content) };
}

function sha256(content: Uint8Array): string {
  return createHash("sha256").update(content).digest("hex");
}

async function checkCandidates(candidates: AssetCandidate[]): Promise<void> {
  const drift: string[] = [];
  for (const candidate of candidates) {
    const target = path.join(targetRoot, candidate.path);
    try {
      const current = await readFile(target);
      if (!current.equals(candidate.content)) {
        drift.push(`${candidate.path}: expected ${candidate.sha256}, found ${sha256(current)}`);
      }
    } catch (error: unknown) {
      drift.push(`${candidate.path}: ${errorMessage(error)}`);
    }
  }
  if (drift.length !== 0) {
    throw new Error(
      `embedded explorer assets differ from the deterministic build:\n${drift.join("\n")}`,
    );
  }
  console.log(`verified ${candidates.length} embedded explorer assets`);
}

async function writeCandidates(candidates: AssetCandidate[]): Promise<void> {
  await mkdir(targetRoot, { recursive: true, mode: 0o755 });
  for (const candidate of candidates) {
    await writeCandidate(candidate);
  }
  await checkCandidates(candidates);
}

async function writeCandidateFiles(candidates: AssetCandidate[]): Promise<void> {
  for (const candidate of candidates) {
    const target = path.join(targetRoot, `${candidate.path}.candidate`);
    await assertTemporaryMissing(target);
    await writeFile(target, candidate.content, { flag: "wx", mode: 0o644 });
    console.log(`${candidate.path}.candidate ${candidate.bytes} ${candidate.sha256}`);
  }
}

async function writeCandidate(candidate: AssetCandidate): Promise<void> {
  const target = path.join(targetRoot, candidate.path);
  const temporary = `${target}.task062-${process.pid}`;
  await assertTemporaryMissing(temporary);
  await writeFile(temporary, candidate.content, { flag: "wx", mode: 0o644 });
  await rename(temporary, target);
  console.log(`${candidate.path} ${candidate.bytes} ${candidate.sha256}`);
}

async function assertTemporaryMissing(temporary: string): Promise<void> {
  try {
    await stat(temporary);
  } catch (error: unknown) {
    if (isMissing(error)) {
      return;
    }
    throw error;
  }
  throw new Error(`temporary asset already exists: ${temporary}`);
}

function isMissing(error: unknown): boolean {
  return error instanceof Error && "code" in error && error.code === "ENOENT";
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "unknown read error";
}

await main();
