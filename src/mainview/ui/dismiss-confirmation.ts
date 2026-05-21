type DismissConfirmationOptions = {
  active: boolean;
  onDismiss: () => void;
};

export function dismissConfirmation(node: HTMLElement, options: DismissConfirmationOptions) {
  let current = options;

  function handlePointerDown(event: PointerEvent) {
    if (!current.active || node.contains(event.target as Node | null)) return;
    current.onDismiss();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!current.active || event.key !== "Escape") return;
    current.onDismiss();
  }

  document.addEventListener("pointerdown", handlePointerDown, true);
  document.addEventListener("keydown", handleKeydown, true);

  return {
    update(next: DismissConfirmationOptions) {
      current = next;
    },
    destroy() {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("keydown", handleKeydown, true);
    },
  };
}
