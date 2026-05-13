export type KeyboardShortcutPart = {
	icon?: "command" | "shift";
	label: string;
	text: string;
};

const SYMBOL_TOKENS: Record<string, KeyboardShortcutPart> = {
	"⌘": { icon: "command", label: "Command", text: "⌘" },
	"⇧": { icon: "shift", label: "Shift", text: "⇧" },
	"⌥": { label: "Option", text: "⌥" },
	"⌃": { label: "Control", text: "⌃" },
	"↵": { label: "Return", text: "↵" },
};

const WORD_TOKENS: Record<string, KeyboardShortcutPart> = {
	alt: { label: "Option", text: "⌥" },
	cmd: { icon: "command", label: "Command", text: "⌘" },
	command: { icon: "command", label: "Command", text: "⌘" },
	commandorcontrol: { icon: "command", label: "Command or Control", text: "⌘" },
	control: { label: "Control", text: "⌃" },
	ctrl: { label: "Control", text: "⌃" },
	enter: { label: "Return", text: "↵" },
	esc: { label: "Escape", text: "Esc" },
	escape: { label: "Escape", text: "Esc" },
	option: { label: "Option", text: "⌥" },
	return: { label: "Return", text: "↵" },
	shift: { icon: "shift", label: "Shift", text: "⇧" },
};

function createKeyboardShortcutIcon(icon: NonNullable<KeyboardShortcutPart["icon"]>): SVGSVGElement {
	const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
	svg.setAttribute("aria-hidden", "true");
	svg.setAttribute("class", "ui-kbd-icon");
	svg.setAttribute("fill", "none");
	svg.setAttribute("focusable", "false");
	svg.setAttribute("stroke", "currentColor");
	svg.setAttribute("stroke-linecap", "round");
	svg.setAttribute("stroke-linejoin", "round");
	svg.setAttribute("stroke-width", "2");
	svg.setAttribute("viewBox", "0 0 24 24");
	const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
	path.setAttribute(
		"d",
		icon === "command"
			? "M15 6v12a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3V6a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3"
			: "M9 19a1 1 0 0 0 1 1h4a1 1 0 0 0 1-1v-6a1 1 0 0 1 1-1h3.293a.707.707 0 0 0 .5-1.207l-7.086-7.086a1 1 0 0 0-1.414 0l-7.086 7.086a.707.707 0 0 0 .5 1.207H8a1 1 0 0 1 1 1z",
	);
	svg.append(path);
	return svg;
}

function normalizeToken(token: string): KeyboardShortcutPart {
	const trimmed = token.trim();
	const directSymbol = SYMBOL_TOKENS[trimmed];
	if (directSymbol) return directSymbol;
	const wordToken = WORD_TOKENS[trimmed.toLowerCase()];
	if (wordToken) return wordToken;
	return {
		label: trimmed.length === 1 ? trimmed.toUpperCase() : trimmed,
		text: trimmed.length === 1 ? trimmed.toUpperCase() : trimmed,
	};
}

export function getKeyboardShortcutParts(shortcut: string): KeyboardShortcutPart[] {
	if (!shortcut.trim()) return [];
	if (shortcut.includes("+")) {
		return shortcut.split("+").filter(Boolean).map(normalizeToken);
	}
	const parts: KeyboardShortcutPart[] = [];
	for (const character of shortcut.trim()) {
		parts.push(normalizeToken(character));
	}
	return parts;
}

export function appendKeyboardShortcutParts(target: HTMLElement, shortcut: string): void {
	for (const part of getKeyboardShortcutParts(shortcut)) {
		const key = document.createElement("span");
		key.className = "ui-kbd-key";
		key.setAttribute("aria-label", part.label);
		if (part.icon) {
			key.append(createKeyboardShortcutIcon(part.icon));
		} else {
			key.textContent = part.text;
		}
		target.append(key);
	}
}
