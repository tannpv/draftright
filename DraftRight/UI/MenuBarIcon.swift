import AppKit

/// Builds the menu-bar status image: the app's SF Symbol tinted by backend
/// status, optionally with a small red "update available" badge dot
/// composited into the top-right corner.
///
/// This is the macOS analog of Windows' `TrayIconBadge.WithDot` — the app is
/// menu-bar-only (`LSUIElement`/`.accessory`), so there's no Dock tile to
/// badge; the always-visible surface is the `MenuBarExtra` icon.
///
/// The result is left non-template (`isTemplate = false`) on purpose: the
/// menu bar would otherwise flatten it to monochrome, dropping both the
/// status tint and the red badge. The icon is already status-coloured, so
/// opting out of template tinting matches existing intent.
enum MenuBarIcon {
    /// red-500, same dot colour Windows uses for parity.
    private static let badgeColor = NSColor(red: 239 / 255, green: 68 / 255, blue: 68 / 255, alpha: 1)

    static func image(symbolName: String,
                      tint: NSColor,
                      showsBadge: Bool,
                      pointSize: CGFloat = 16) -> NSImage {
        let config = NSImage.SymbolConfiguration(pointSize: pointSize, weight: .regular)
        let symbol = NSImage(systemSymbolName: symbolName, accessibilityDescription: "DraftRight")?
            .withSymbolConfiguration(config)
        let size = symbol?.size ?? NSSize(width: pointSize + 4, height: pointSize + 4)

        let result = NSImage(size: size)
        result.lockFocus()

        // Tint the symbol: draw it, then flood its opaque pixels with the
        // status colour via .sourceAtop.
        let rect = NSRect(origin: .zero, size: size)
        symbol?.draw(in: rect)
        tint.set()
        rect.fill(using: .sourceAtop)

        if showsBadge {
            // Dot ~45% of the icon, in the top-right corner with a 1px inset.
            let d = max(5, min(size.width, size.height) * 0.45)
            let x = size.width - d - 1
            let y = size.height - d - 1
            // White ring first so the dot reads on any menu-bar background.
            NSColor.white.setFill()
            NSBezierPath(ovalIn: NSRect(x: x - 1, y: y - 1, width: d + 2, height: d + 2)).fill()
            badgeColor.setFill()
            NSBezierPath(ovalIn: NSRect(x: x, y: y, width: d, height: d)).fill()
        }

        result.unlockFocus()
        result.isTemplate = false
        return result
    }
}
