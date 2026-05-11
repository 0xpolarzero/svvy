export type KeybindingId =
  | "commandPalette.open"
  | "quickOpen.open"
  | "session.new"
  | "session.dumb"
  | "sidebar.toggle";

export type KeybindingEvent = {
  defaultPrevented: boolean;
  altKey: boolean;
  metaKey: boolean;
  ctrlKey: boolean;
  shiftKey: boolean;
  key: string;
};

export type KeybindingDefinition = {
  id: KeybindingId;
  label: string;
  shortcut: string;
  displayShortcut: string;
  accelerator: string;
  match: (event: KeybindingEvent) => boolean;
};

export type AppMenuAction = KeybindingId;

function hasCommandModifier(event: Pick<KeybindingEvent, "metaKey" | "ctrlKey">): boolean {
  return event.metaKey || event.ctrlKey;
}

function isPlainCommandKey(event: KeybindingEvent, key: string): boolean {
  return (
    !event.defaultPrevented &&
    !event.altKey &&
    !event.shiftKey &&
    hasCommandModifier(event) &&
    event.key.toLowerCase() === key
  );
}

function isShiftCommandKey(event: KeybindingEvent, key: string): boolean {
  return (
    !event.defaultPrevented &&
    !event.altKey &&
    event.shiftKey &&
    hasCommandModifier(event) &&
    event.key.toLowerCase() === key
  );
}

export const KEYBINDINGS: Record<KeybindingId, KeybindingDefinition> = {
  "commandPalette.open": {
    id: "commandPalette.open",
    label: "Open Command Palette",
    shortcut: "Cmd+Shift+P",
    displayShortcut: "⌘⇧P",
    accelerator: "CommandOrControl+Shift+P",
    match: (event) => isShiftCommandKey(event, "p"),
  },
  "quickOpen.open": {
    id: "quickOpen.open",
    label: "Open Quick Open",
    shortcut: "Cmd+P",
    displayShortcut: "⌘P",
    accelerator: "CommandOrControl+P",
    match: (event) => isPlainCommandKey(event, "p"),
  },
  "session.new": {
    id: "session.new",
    label: "New Session",
    shortcut: "Cmd+N",
    displayShortcut: "⌘N",
    accelerator: "CommandOrControl+N",
    match: (event) => isPlainCommandKey(event, "n"),
  },
  "session.dumb": {
    id: "session.dumb",
    label: "New Dumb Session",
    shortcut: "Cmd+Shift+N",
    displayShortcut: "⌘⇧N",
    accelerator: "CommandOrControl+Shift+N",
    match: (event) => isShiftCommandKey(event, "n"),
  },
  "sidebar.toggle": {
    id: "sidebar.toggle",
    label: "Toggle Sidebar",
    shortcut: "Cmd+B",
    displayShortcut: "⌘B",
    accelerator: "CommandOrControl+B",
    match: (event) => isPlainCommandKey(event, "b"),
  },
};

export function getKeybinding(id: KeybindingId): KeybindingDefinition {
  return KEYBINDINGS[id];
}

export function getKeybindingShortcut(id: KeybindingId): string {
  return getKeybinding(id).shortcut;
}

export function getKeybindingDisplayShortcut(id: KeybindingId): string {
  return getKeybinding(id).displayShortcut;
}

export function getKeybindingAccelerator(id: KeybindingId): string {
  return getKeybinding(id).accelerator;
}

export function isAppMenuAction(value: unknown): value is AppMenuAction {
  return typeof value === "string" && value in KEYBINDINGS;
}

export function matchesKeybinding(event: KeybindingEvent, id: KeybindingId): boolean {
  return getKeybinding(id).match(event);
}
