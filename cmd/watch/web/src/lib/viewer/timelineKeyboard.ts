export function isTextEntryTarget(target: EventTarget | null): boolean {
  if (!target || typeof target !== "object" || !("tagName" in target)) {
    return false;
  }

  const element = target as HTMLElement;
  if (element.isContentEditable) {
    return true;
  }

  const tagName = element.tagName.toUpperCase();
  if (tagName === "TEXTAREA" || tagName === "SELECT") {
    return true;
  }

  if (tagName !== "INPUT") {
    return false;
  }

  return (element as HTMLInputElement).type !== "range";
}

export function shouldHandleTimelineKeydown(event: KeyboardEvent): boolean {
  if (
    event.defaultPrevented
    || event.altKey
    || event.ctrlKey
    || event.metaKey
    || event.shiftKey
    || isTextEntryTarget(event.target)
  ) {
    return false;
  }

  return event.key === "ArrowLeft" || event.key === "ArrowRight";
}
