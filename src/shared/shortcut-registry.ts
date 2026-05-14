export type ShortcutScope =
  | "global"
  | "workspace-shell"
  | "focused-pane"
  | "dialog"
  | "input";

export type ShortcutInputPolicy = "suppress-while-typing" | "allow-while-typing";

export type ShortcutActionId =
  | "commandPalette.open"
  | "quickOpen.open"
  | "session.new"
  | "session.dumb"
  | "sidebar.toggle"
  | "dialog.close"
  | "commandPalette.submit"
  | "commandPalette.submitFocusedPane"
  | "composer.submit";

export type AppMenuAction = Extract<
  ShortcutActionId,
  "commandPalette.open" | "quickOpen.open" | "session.new" | "session.dumb" | "sidebar.toggle"
>;

export type ShortcutDefinition = {
  id: ShortcutActionId;
  label: string;
  hotkey: string;
  readableShortcut: string;
  compactShortcut: string;
  accelerator: string | null;
  scope: ShortcutScope;
  inputPolicy: ShortcutInputPolicy;
  commandActionId?: string;
};

export const SHORTCUTS = {
  "commandPalette.open": {
    id: "commandPalette.open",
    label: "Open Command Palette",
    hotkey: "Mod+Shift+P",
    readableShortcut: "Cmd+Shift+P",
    compactShortcut: "⌘⇧P",
    accelerator: "CommandOrControl+Shift+P",
    scope: "global",
    inputPolicy: "allow-while-typing",
    commandActionId: "commandPalette.open",
  },
  "quickOpen.open": {
    id: "quickOpen.open",
    label: "Open Quick Open",
    hotkey: "Mod+P",
    readableShortcut: "Cmd+P",
    compactShortcut: "⌘P",
    accelerator: "CommandOrControl+P",
    scope: "global",
    inputPolicy: "allow-while-typing",
    commandActionId: "quickOpen.open",
  },
  "session.new": {
    id: "session.new",
    label: "New Session",
    hotkey: "Mod+N",
    readableShortcut: "Cmd+N",
    compactShortcut: "⌘N",
    accelerator: "CommandOrControl+N",
    scope: "workspace-shell",
    inputPolicy: "allow-while-typing",
    commandActionId: "session.new",
  },
  "session.dumb": {
    id: "session.dumb",
    label: "New Dumb Session",
    hotkey: "Mod+Shift+N",
    readableShortcut: "Cmd+Shift+N",
    compactShortcut: "⌘⇧N",
    accelerator: "CommandOrControl+Shift+N",
    scope: "workspace-shell",
    inputPolicy: "allow-while-typing",
    commandActionId: "session.dumb",
  },
  "sidebar.toggle": {
    id: "sidebar.toggle",
    label: "Toggle Sidebar",
    hotkey: "Mod+B",
    readableShortcut: "Cmd+B",
    compactShortcut: "⌘B",
    accelerator: "CommandOrControl+B",
    scope: "workspace-shell",
    inputPolicy: "allow-while-typing",
    commandActionId: "sidebar.toggle",
  },
  "dialog.close": {
    id: "dialog.close",
    label: "Close Dialog",
    hotkey: "Escape",
    readableShortcut: "Escape",
    compactShortcut: "Esc",
    accelerator: null,
    scope: "dialog",
    inputPolicy: "allow-while-typing",
  },
  "commandPalette.submit": {
    id: "commandPalette.submit",
    label: "Run Selected Command",
    hotkey: "Enter",
    readableShortcut: "Enter",
    compactShortcut: "↵",
    accelerator: null,
    scope: "dialog",
    inputPolicy: "allow-while-typing",
  },
  "commandPalette.submitFocusedPane": {
    id: "commandPalette.submitFocusedPane",
    label: "Run In Focused Pane",
    hotkey: "Mod+Enter",
    readableShortcut: "Cmd+Enter",
    compactShortcut: "⌘↵",
    accelerator: null,
    scope: "dialog",
    inputPolicy: "allow-while-typing",
  },
  "composer.submit": {
    id: "composer.submit",
    label: "Send Prompt",
    hotkey: "Enter",
    readableShortcut: "Enter",
    compactShortcut: "↵",
    accelerator: null,
    scope: "input",
    inputPolicy: "allow-while-typing",
  },
} satisfies Record<ShortcutActionId, ShortcutDefinition>;

export function getShortcut(id: ShortcutActionId): ShortcutDefinition {
  return SHORTCUTS[id];
}

export function getShortcutHotkey(id: ShortcutActionId): string {
  return getShortcut(id).hotkey;
}

export function getShortcutReadable(id: ShortcutActionId): string {
  return getShortcut(id).readableShortcut;
}

export function getShortcutCompact(id: ShortcutActionId): string {
  return getShortcut(id).compactShortcut;
}

export function getShortcutAccelerator(id: ShortcutActionId): string | null {
  return getShortcut(id).accelerator;
}

export function shouldShortcutIgnoreInputs(id: ShortcutActionId): boolean {
  return getShortcut(id).inputPolicy === "suppress-while-typing";
}

export function isAppMenuAction(value: unknown): value is AppMenuAction {
  return typeof value === "string" && value in SHORTCUTS && !!SHORTCUTS[value as ShortcutActionId].accelerator;
}
