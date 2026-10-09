export function getRescanShortcutAction(event, isMacOS) {
    if (event.altKey || event.isComposing || event.key.toLowerCase() !== "r") {
        return null;
    }

    const hasPrimaryModifier = (event.ctrlKey && !event.metaKey) ||
        (isMacOS && event.metaKey && !event.ctrlKey);
    return hasPrimaryModifier ? Boolean(event.shiftKey) : null;
}

export function registerRescanShortcuts(target, isMacOS, isBlocked, onRescan) {
    const handleKeydown = event => {
        const supportsCommand = typeof isMacOS === "function" ? isMacOS() : isMacOS;
        const hard = getRescanShortcutAction(event, supportsCommand);
        if (hard === null) {
            return;
        }

        event.preventDefault();
        if (event.repeat || isBlocked()) {
            return;
        }

        onRescan(hard);
    };

    target.addEventListener("keydown", handleKeydown, true);
    return () => target.removeEventListener("keydown", handleKeydown, true);
}
