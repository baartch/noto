// Package tui implements the Bubble Tea TUI models, views, and update loop for Noto.
//
// Bubbles components are used where they satisfy UI requirements, including the
// Help bubble for footer/expanded keybinding help. Custom components (e.g., picker
// rendering) are retained only when Bubbles does not support required behaviors
// such as mixed active markers, inline filtering, and custom footers.
package tui
