import { existsSync } from "node:fs";
import { join } from "node:path";
import { dlopen, FFIType, suffix, type Pointer } from "bun:ffi";

type WindowControlsLibrary = {
  symbols: {
    svvyPositionTrafficLights: (windowPointer: Pointer, leading: number, top: number) => void;
  };
};

let nativeWindowControls: WindowControlsLibrary | null | undefined;

function resolveNativeWindowControlsPath(): string | null {
  const candidates = [
    join(process.cwd(), `libSvvyWindowControls.${suffix}`),
    join(process.cwd(), "build", "native", `libSvvyWindowControls.${suffix}`),
    join(import.meta.dir, "..", "..", "build", "native", `libSvvyWindowControls.${suffix}`),
  ];

  return candidates.find((candidate) => existsSync(candidate)) ?? null;
}

function loadNativeWindowControls(): WindowControlsLibrary | null {
  if (nativeWindowControls !== undefined) {
    return nativeWindowControls;
  }

  if (process.platform !== "darwin") {
    nativeWindowControls = null;
    return nativeWindowControls;
  }

  const libraryPath = resolveNativeWindowControlsPath();
  if (!libraryPath) {
    console.warn(
      "Native window-controls library was not found; using default macOS traffic-light placement.",
    );
    nativeWindowControls = null;
    return nativeWindowControls;
  }

  nativeWindowControls = dlopen(libraryPath, {
    svvyPositionTrafficLights: {
      args: [FFIType.ptr, FFIType.f64, FFIType.f64],
      returns: FFIType.void,
    },
  }) as WindowControlsLibrary;
  return nativeWindowControls;
}

export function positionNativeTrafficLights(
  windowPointer: Pointer | undefined,
  options: { leading: number; top: number },
): void {
  if (process.platform !== "darwin" || !windowPointer) {
    return;
  }

  try {
    loadNativeWindowControls()?.symbols.svvyPositionTrafficLights(
      windowPointer,
      options.leading,
      options.top,
    );
  } catch (error) {
    console.warn("Unable to position native macOS traffic lights:", error);
  }
}
