#!/usr/bin/env bun

import { existsSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { spawnSync } from "node:child_process";

const projectRoot = join(import.meta.dir, "..");
const sourcePath = join(projectRoot, "src", "native", "window-controls.m");
const outputPath = join(projectRoot, "build", "native", "libSvvyWindowControls.dylib");

if (process.platform !== "darwin") {
  process.exit(0);
}

mkdirSync(dirname(outputPath), { recursive: true });

const result = spawnSync(
  "clang",
  [
    "-dynamiclib",
    "-fobjc-arc",
    "-framework",
    "AppKit",
    "-o",
    outputPath,
    sourcePath,
  ],
  {
    cwd: projectRoot,
    stdio: "inherit",
  },
);

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}

if (!existsSync(outputPath)) {
  console.error(`Native window-controls build did not produce ${outputPath}`);
  process.exit(1);
}

console.log(`Built native window-controls library at ${outputPath}`);
