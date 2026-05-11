#!/usr/bin/env swift

import AppKit

// Generate FOREGROUND ONLY (white pencil on transparent background)
// For Android adaptive icons
func generateForeground(size: Int) -> NSImage {
    let img = NSImage(size: NSSize(width: size, height: size))
    img.lockFocus()

    let s = CGFloat(size)

    // Transparent background — only draw the pencil icon
    let symbolConfig = NSImage.SymbolConfiguration(pointSize: s * 0.35, weight: .medium)
    if let symbol = NSImage(systemSymbolName: "pencil.and.outline", accessibilityDescription: nil)?
        .withSymbolConfiguration(symbolConfig) {
        let symbolSize = symbol.size
        let x = (s - symbolSize.width) / 2
        let y = (s - symbolSize.height) / 2
        symbol.draw(in: NSRect(x: x, y: y, width: symbolSize.width, height: symbolSize.height),
                    from: .zero, operation: .sourceOver, fraction: 1.0)
        // Tint white
        NSColor.white.setFill()
        NSRect(x: x, y: y, width: symbolSize.width, height: symbolSize.height).fill(using: .sourceAtop)
    }

    img.unlockFocus()
    return img
}

let icon = generateForeground(size: 1024)
guard let tiff = icon.tiffRepresentation,
      let rep = NSBitmapImageRep(data: tiff),
      let png = rep.representation(using: .png, properties: [:]) else {
    print("Failed")
    exit(1)
}

let path = "/opt/openAi/DraftRight/DraftRightMobile/assets/icon_foreground.png"
try! png.write(to: URL(fileURLWithPath: path))
print("Foreground icon generated at \(path)")
