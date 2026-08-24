# World-class practice upgrades

## Terminal cells are not pixels
- Measure rendered width in terminal cells, not bytes/runes; account for wide graphemes and combining characters where supported.
- Use consistent display-cell width for truncation, padding, tables, and cursor positioning.
- Keep fallbacks for essential Unicode indicators.

## Architecture and lifecycle
- Keep domain state independent from the TUI framework.
- Use an action/command registry as the semantic source for keybindings, palette, help, and context actions.
- Separate focus, selection, navigation history, and edit mode.
- Long work must run outside the update loop, propagate cancellation, and return messages/events.
- Restore raw mode, cursor, mouse mode, alt screen, and terminal state on normal and panic/error exits.

## Capability and performance
- Treat mouse/TrueColor/Nerd Font as optional enhancement; test SSH/tmux/high latency.
- Render only visible data for large lists/logs and recompute layout—not domain data—on resize.
- Scope animation ticks to components that need them.

## Testing
- Unit-test reducer/state machines, command enablement, and key conflicts.
- Golden-test representative size classes and no-color mode.
- Test resize during editing, cancellation, failure, and terminal cleanup; benchmark hot render/update paths before exotic optimization.
