import assert from "node:assert/strict";
import test from "node:test";
import {getRescanShortcutAction, registerRescanShortcuts} from "../frontend/rescan_shortcuts.js";

function keyboardEvent(overrides = {}) {
    return {
        key: "r",
        ctrlKey: false,
        metaKey: false,
        shiftKey: false,
        altKey: false,
        isComposing: false,
        repeat: false,
        ...overrides
    };
}

test("Ctrl shortcuts request normal and hard rescans", () => {
    assert.equal(getRescanShortcutAction(keyboardEvent({ctrlKey: true}), false), false);
    assert.equal(getRescanShortcutAction(keyboardEvent({ctrlKey: true, shiftKey: true}), false), true);
});

test("Command shortcuts are macOS aliases", () => {
    assert.equal(getRescanShortcutAction(keyboardEvent({metaKey: true}), true), false);
    assert.equal(getRescanShortcutAction(keyboardEvent({metaKey: true, shiftKey: true}), true), true);
    assert.equal(getRescanShortcutAction(keyboardEvent({metaKey: true}), false), null);
    assert.equal(getRescanShortcutAction(keyboardEvent({ctrlKey: true}), true), false);
});

test("unrelated modifiers and composing input are ignored", () => {
    assert.equal(getRescanShortcutAction(keyboardEvent(), false), null);
    assert.equal(getRescanShortcutAction(keyboardEvent({ctrlKey: true, altKey: true}), false), null);
    assert.equal(getRescanShortcutAction(keyboardEvent({ctrlKey: true, metaKey: true}), true), null);
    assert.equal(getRescanShortcutAction(keyboardEvent({ctrlKey: true, key: "s"}), false), null);
    assert.equal(getRescanShortcutAction(keyboardEvent({ctrlKey: true, isComposing: true}), false), null);
});

test("registered shortcuts prevent reload, ignore repeats, and respect busy state", () => {
    let listener;
    let capture = false;
    const calls = [];
    const target = {
        addEventListener(type, callback, useCapture) {
            assert.equal(type, "keydown");
            listener = callback;
            capture = useCapture;
        },
        removeEventListener(type, callback, useCapture) {
            assert.equal(type, "keydown");
            assert.equal(callback, listener);
            assert.equal(useCapture, true);
            listener = undefined;
        }
    };
    let blocked = false;
    const unregister = registerRescanShortcuts(target, false, () => blocked, hard => calls.push(hard));

    assert.equal(capture, true);
    let prevented = 0;
    const dispatch = overrides => listener({
        ...keyboardEvent({ctrlKey: true}),
        ...overrides,
        preventDefault() { prevented++; }
    });
    dispatch({});
    dispatch({shiftKey: true});
    dispatch({repeat: true});
    blocked = true;
    dispatch({});

    assert.equal(prevented, 4);
    assert.deepEqual(calls, [false, true]);
    unregister();
    assert.equal(listener, undefined);
});

test("registered shortcut observes platform detection after startup", () => {
    let listener;
    let isMacOS = false;
    const calls = [];
    const target = {
        addEventListener(type, callback) { listener = callback; },
        removeEventListener() { listener = undefined; }
    };
    registerRescanShortcuts(target, () => isMacOS, () => false, hard => calls.push(hard));

    let prevented = 0;
    const event = {
        ...keyboardEvent({metaKey: true}),
        preventDefault() { prevented++; }
    };
    listener(event);
    isMacOS = true;
    listener(event);

    assert.equal(prevented, 1);
    assert.deepEqual(calls, [false]);
});
