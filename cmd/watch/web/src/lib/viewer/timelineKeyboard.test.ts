import { describe, expect, it } from 'vitest';
import { isTextEntryTarget, shouldHandleTimelineKeydown } from './timelineKeyboard';

function target(tagName: string, props: Record<string, unknown> = {}): EventTarget {
  return {
    tagName,
    isContentEditable: false,
    ...props,
  } as unknown as EventTarget;
}

function keydown(key: string, props: Partial<KeyboardEvent> = {}): KeyboardEvent {
  return {
    key,
    defaultPrevented: false,
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    target: null,
    ...props,
  } as KeyboardEvent;
}

describe('isTextEntryTarget', () => {
  it('treats text editing controls as text entry targets', () => {
    expect(isTextEntryTarget(target('input', { type: 'text' }))).toBe(true);
    expect(isTextEntryTarget(target('textarea'))).toBe(true);
    expect(isTextEntryTarget(target('select'))).toBe(true);
    expect(isTextEntryTarget(target('div', { isContentEditable: true }))).toBe(true);
  });

  it('allows range inputs and normal UI targets to use timeline shortcuts', () => {
    expect(isTextEntryTarget(target('input', { type: 'range' }))).toBe(false);
    expect(isTextEntryTarget(target('button'))).toBe(false);
    expect(isTextEntryTarget(null)).toBe(false);
  });
});

describe('shouldHandleTimelineKeydown', () => {
  it('handles unmodified left and right arrow keys outside text entry targets', () => {
    expect(shouldHandleTimelineKeydown(keydown('ArrowLeft'))).toBe(true);
    expect(shouldHandleTimelineKeydown(keydown('ArrowRight', { target: target('button') }))).toBe(true);
    expect(shouldHandleTimelineKeydown(keydown('ArrowRight', { target: target('input', { type: 'range' }) }))).toBe(true);
  });

  it('ignores other keys, modified keys, and dropdown focus', () => {
    expect(shouldHandleTimelineKeydown(keydown('Enter'))).toBe(false);
    expect(shouldHandleTimelineKeydown(keydown('ArrowLeft', { metaKey: true }))).toBe(false);
    expect(shouldHandleTimelineKeydown(keydown('ArrowRight', { target: target('select') }))).toBe(false);
  });
});
