# World-class practice upgrades

## Host-first integration
- Verify version-sensitive API behavior against current official Obsidian docs.
- Prefer lifecycle helpers such as registered events/DOM events/intervals/editor extensions so resources are disposed on unload.
- Keep `onload()` lightweight and avoid fragile undocumented DOM internals when public APIs exist.

## CSS/theme compatibility
- Scope CSS under a stable plugin root and use Obsidian CSS variables for surfaces, text, interaction, radii, and typography.
- Avoid global `!important`, Default-theme color assumptions, and plugin-owned replacement of host accent.
- Test Default light/dark, materially different community themes, and increased interface font size.

## Workspace/editor lifecycle
- Preserve useful view state across leaf resize/rearrangement; do not remount expensive UI just because a breakpoint changed.
- Clean up framework roots, observers, workers, timers, subscriptions, and custom DOM.
- Editor decorations must avoid synchronous vault-wide work and layout thrash.

## Vault and mobile
- Use stable vault/file APIs for mobile-compatible paths.
- If an auxiliary DB/index exists, document canonical vs derived state and rebuild/invalidation.
- Test large vaults and rapid file changes.
- Treat mobile as a capability profile; avoid Node/Electron-only APIs on mobile paths and never require hover.

## Interoperability
- Important actions should exist as commands; avoid stealing common default hotkeys.
- Minimize global event interception and editor decoration conflicts with other plugins.
